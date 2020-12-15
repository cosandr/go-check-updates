package main

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cosandr/go-check-updates/api"
	log "github.com/sirupsen/logrus"
)

func generateUpdates(num int) api.UpdatesList {
	updates := make(api.UpdatesList, 0)
	for i := 1; i < num+1; i++ {
		old := rand.Intn(10) + 1
		updates = append(updates, api.Update{
			Pkg:    fmt.Sprintf("Update %d", i),
			OldVer: fmt.Sprintf("%d", old),
			NewVer: fmt.Sprintf("%d.%d", old, rand.Intn(3)+1),
			Repo:   fmt.Sprintf("Repo %d", i),
		})
	}
	return updates
}

func TestDiscordDiffUpdates(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	old := api.UpdatesList{
		{
			Pkg:    "pkg1",
			NewVer: "v1",
			Repo:   "updates",
		},
		{
			Pkg:    "pkg2",
			NewVer: "v2",
			Repo:   "updates",
		},
	}
	// Test with same updates
	new := make(api.UpdatesList, len(old))
	copy(new, old)
	actual := diffUpdates(&old, &new)
	if len(actual) != 0 {
		t.Errorf("Expected 0 updates, got %v", actual)
	}
	// Add an update
	add := api.Update{
		Pkg:    "pkg3",
		NewVer: "v3",
		Repo:   "updates",
	}
	new = append(new, add)
	actual = diffUpdates(&old, &new)
	if len(actual) != 1 && !actual[0].Equals(&add) {
		t.Errorf("Expected %v, got %v", add, actual)
	}
	// Remove an update
	new = api.UpdatesList{
		{
			Pkg:    "pkg1",
			NewVer: "v1",
			Repo:   "updates",
		},
	}
	actual = diffUpdates(&old, &new)
	if len(actual) != 0 {
		t.Errorf("Expected 0 updates, got %v", actual)
	}
}

func TestDiscordSendUpdatesNotificationDiff(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	args.WebhookURL = os.Getenv("WEBHOOK_URL")
	args.NotifyFormat = os.Getenv("NOTIFY_FORMAT")
	args.NotifyDelta = true
	if args.NotifyFormat == "" {
		args.NotifyFormat = "2006/01/02 15:04"
	}
	cache = NewInternalCache()
	cache.f.Checked = time.Now().Format(time.RFC3339)
	cache.f.Updates = api.UpdatesList{
		{
			Pkg:    "pkg1",
			NewVer: "v1",
			Repo:   "updates",
		},
		{
			Pkg:    "pkg2",
			NewVer: "v2",
			Repo:   "updates",
		},
	}
	// Test with same updates
	prevNotifyUpdates = make(api.UpdatesList, len(cache.f.Updates))
	copy(prevNotifyUpdates, cache.f.Updates)
	if err := sendUpdatesNotification(); err != nil {
		t.Error(err)
		return
	}
	// Add an update
	cache.f.Updates = append(cache.f.Updates, api.Update{
		Pkg:    "pkg3",
		NewVer: "v3",
		Repo:   "updates",
	})
	if err := sendUpdatesNotification(); err != nil {
		t.Error(err)
		return
	}
	// Remove an update
	cache.f.Updates = api.UpdatesList{
		{
			Pkg:    "pkg1",
			NewVer: "v1",
			Repo:   "updates",
		},
	}
	if err := sendUpdatesNotification(); err != nil {
		t.Error(err)
		return
	}
}

func TestDiscordSendUpdatesNotification(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	args.WebhookURL = os.Getenv("WEBHOOK_URL")
	args.NotifyFormat = os.Getenv("NOTIFY_FORMAT")
	args.NotifyDelta = false
	if args.NotifyFormat == "" {
		args.NotifyFormat = "2006/01/02 15:04"
	}
	cache = NewInternalCache()
	// Should send fields, detailed description, names only, number only
	for _, num := range []int{15, 30, 150, 9001} {
		cache.f.Updates = generateUpdates(num)
		cache.f.Checked = time.Now().Format(time.RFC3339)
		if err := sendUpdatesNotification(); err != nil {
			t.Error(err)
			return
		}
	}
}

func TestDiscordSendWebhook(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	args.WebhookURL = os.Getenv("WEBHOOK_URL")
	embed := discordgo.MessageEmbed{
		Title:       "Test Webhook Title",
		Description: "Test Webhook Description",
	}
	err := sendWebhook(&embed)
	if err != nil {
		t.Error(err)
	}
}

func TestDiscordSendWebhookWithMessage(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	args.WebhookURL = os.Getenv("WEBHOOK_URL")
	re := regexp.MustCompile(`https://discord(?:app)?\.com/api/webhooks/(\d{18})/\S+`)
	m := re.FindStringSubmatch(args.WebhookURL)
	if m == nil {
		t.Error("bad webhook URL")
		return
	}
	embed := discordgo.MessageEmbed{
		Title:       "Test Webhook Title",
		Description: "Test Webhook Description",
	}
	msg, err := sendWebhookWithMessage(&embed)
	if err != nil {
		t.Error(err)
		return
	}
	if msg.WebhookID != m[1] {
		t.Errorf("expected webhook ID %s, got %s", m, msg.WebhookID)
	}
}
