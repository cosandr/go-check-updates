package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

// TODO: Real-time updates using cache.Subscribe

// https://discord.com/developers/docs/resources/channel#embed-limits
const (
	embedMaxTitle       = 256
	embedMaxDescription = 2048
	embedMaxFields      = 25
	embedMaxFieldName   = 256
	embedMaxFieldValue  = 1024
	embedMaxFooter      = 2048
	embedMaxAuthordName = 256
	embedMaxTotal       = 6000
	webhookMaxEmbeds    = 10
)

// embedExceedsLimits returns true if embed total character count is too large
func embedExceedsLimits(embed *discordgo.MessageEmbed) bool {
	lenTitle := len(embed.Title)
	if lenTitle > embedMaxTitle {
		log.Debug("embed exceeds title limit")
		return true
	}
	lenDescription := len(embed.Description)
	if lenDescription > embedMaxDescription {
		log.Debug("embed exceeds description limit")
		return true
	}
	total := lenTitle + lenDescription
	if embed.Footer != nil {
		lenText := len(embed.Footer.Text)
		if lenText > embedMaxFooter {
			log.Debug("embed exceeds title limit")
			return true
		}
		total += lenText
	}
	if len(embed.Fields) > 0 {
		for _, f := range embed.Fields {
			if f == nil {
				continue
			}
			lenName := len(f.Name)
			if lenName > embedMaxFieldName {
				log.Debug("embed field exceeds name limit")
				return true
			}
			lenValue := len(f.Value)
			if lenValue > embedMaxFieldValue {
				log.Debug("embed field exceeds value limit")
				return true
			}
			total += lenName + lenValue
		}
	}
	if embed.Author != nil {
		lenName := len(embed.Author.Name)
		if lenName > embedMaxAuthordName {
			log.Debug("embed author exceeds name limit")
			return true
		}
		total += lenName
	}
	return total > embedMaxTotal
}

func sendUpdatesNotification() error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	num := len(cache.f.Updates)
	embed := discordgo.MessageEmbed{
		Title: fmt.Sprintf("%d pending updates for %s", num, hostname),
	}
	if t, err := time.Parse(time.RFC3339, cache.f.Checked); err == nil {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Checked %s", t.Format(args.NotifyFormat)),
		}
	}
	if num <= embedMaxFields {
		log.Debug("adding updates to embed fields")
		embed.Fields = make([]*discordgo.MessageEmbedField, num)
		for i, u := range cache.f.Updates {
			field := discordgo.MessageEmbedField{Name: u.Pkg, Inline: true}
			if u.OldVer != "" {
				field.Value = fmt.Sprintf("%s -> %s", u.OldVer, u.NewVer)
			} else {
				field.Value += fmt.Sprintf("New version %s", u.NewVer)
			}
			embed.Fields[i] = &field
		}
	}
	// Too many for fields or somehow too many characters
	if len(embed.Fields) == 0 || embedExceedsLimits(&embed) {
		log.Debug("adding updates to embed description")
		// Reset fields
		embed.Fields = make([]*discordgo.MessageEmbedField, 0)
		// Write to description
		for _, u := range cache.f.Updates {
			embed.Description += "\n" + u.Pkg
			if u.OldVer != "" {
				embed.Description += fmt.Sprintf(" [%s -> %s]", u.OldVer, u.NewVer)
			} else {
				embed.Description += fmt.Sprintf(" [%s]", u.NewVer)
			}
		}
		embed.Description = strings.TrimSpace(embed.Description)
	}
	// If we still exceed total, try with just update names
	if len(embed.Fields) == 0 && embedExceedsLimits(&embed) {
		log.Debug("embed description too long, trying names only")
		tmp := make([]string, num)
		for i, u := range cache.f.Updates {
			tmp[i] = u.Pkg
		}
		embed.Description = strings.Join(tmp, ", ")
	}
	// STILL exceeding, don't include any names at all
	if len(embed.Fields) == 0 && embedExceedsLimits(&embed) {
		log.Debug("embed still too long, number only")
		embed.Description = ""
	}
	return sendWebhook(&embed)
}

// sendWebhook execute webhook without waiting for the message
func sendWebhook(embed *discordgo.MessageEmbed) error {
	if args.WebhookURL == "" {
		return errors.New("no webhook URL configured")
	}
	w := discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	}
	rBody, err := json.Marshal(&w)
	if err != nil {
		return err
	}
	resp, err := http.Post(args.WebhookURL, "application/json", bytes.NewBuffer(rBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		log.Debugf("POST response: %v", resp)
		return fmt.Errorf("cannot send webhook, server responded with %s", resp.Status)
	}
	return nil
}

// sendWebhookWithMessage executes webhook and returns a pointer to the newly created message
func sendWebhookWithMessage(embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	if args.WebhookURL == "" {
		return nil, errors.New("no webhook URL configured")
	}
	w := discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	}
	rBody, err := json.Marshal(&w)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", args.WebhookURL, bytes.NewBuffer(rBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	q := req.URL.Query()
	q.Add("wait", "true")
	req.URL.RawQuery = q.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Debugf("POST response: %v", resp)
		return nil, fmt.Errorf("cannot send webhook, server responded with %s", resp.Status)
	}
	msgBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read Discord response body: %v", err)
	}
	var msg discordgo.Message
	if err := json.Unmarshal(msgBody, &msg); err != nil {
		return nil, fmt.Errorf("cannot unmarshal into Message struct: %v", err)
	}
	return &msg, nil
}
