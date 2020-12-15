package main

/*
$ checkupdates
libarchive 3.4.0-3 -> 3.4.1-1
libjpeg-turbo 2.0.3-1 -> 2.0.4-1
linux 5.4.6.arch3-1 -> 5.4.7.arch1-1
linux-headers 5.4.6.arch3-1 -> 5.4.7.arch1-1
shellcheck 0.7.0-82 -> 0.7.0-83

$ pikaur -Qua 2>/dev/null
 corefreq-git                          1.70-1               -> 1.71-1
 pikaur                                1.5.7-1              -> 1.5.8-1
*/

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/cosandr/go-check-updates/api"
)

// helper contains data necessary to run an AUR helper
type helper struct {
	name string         // executable name
	args string         // arguments to use when checking for AUR-only updates
	re   *regexp.Regexp // regex pattern for updates
}

type updRes struct {
	upd api.UpdatesList
	err error
}

const pacmanTimeFmt = "2006-01-02T15:04:05-0700" // old format "2006-01-02 15:04"

// Group 1: name
// Group 2: old version
// Group 3: new version
var rePacman = regexp.MustCompile(`(?m)^\s*(\S+)\s+(\S+)\s+->\s+(\S+)\s*$`)

// Group 1: timestamp
// Group 2: action (installed, upgraded, removed)
// Group 3: package
// Group 4: if installed or removed, version
//			if upgraded, <oldVersion> -> <newVersion> (same as pacman -Qu)
var rePacmanLog = regexp.MustCompile(`^\[(\S+)\]\s\[ALPM\]\s(\w+)\s(\S+)\s\((.*)\)$`)

var supportedHelpers = []helper{
	{
		name: "yay",
		args: "-Qua --color=never",
		re:   rePacman,
	},
	{
		name: "paru",
		args: "-Qua --color=never",
		re:   rePacman,
	},
	{
		name: "pikaur",
		args: "-Qua --color=never",
		re:   rePacman,
	},
}

func procPacman(ch chan<- updRes) {
	res := updRes{}
	defer func() {
		ch <- res
	}()
	raw, err := runCmd("checkupdates")
	if err != nil {
		// Exit code 2 is OK, no updates
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() != 2 {
				res.err = err
			}
		}
		return
	}
	res.upd = parsePacmanCheckUpdates(raw, rePacman, "pacman")
}

func procAUR(ch chan<- updRes) {
	res := updRes{}
	defer func() {
		ch <- res
	}()
	if aur.name == "" {
		log.Debug("procAUR: no AUR helper, skipping")
		return
	}
	raw, err := runCmd(aur.name, aur.args)
	if err != nil {
		if aur.name == "paru" {
			// Exit code 1 is OK, no updates
			if exitError, ok := err.(*exec.ExitError); ok {
				if exitError.ExitCode() != 1 {
					res.err = err
					return
				}
			}
		} else {
			res.err = err
			return
		}
	}
	if aur.re == nil {
		res.err = fmt.Errorf("regex for %s is nil", aur.name)
		return
	}
	res.upd = parsePacmanCheckUpdates(raw, aur.re, "aur")
}

// UpdateArch uses checkupdates and (if available) a supported AUR helper to get available updates
func UpdateArch() (updates api.UpdatesList, err error) {
	chPac := make(chan updRes)
	chAUR := make(chan updRes)
	// Run in parallel
	go procPacman(chPac)
	go procAUR(chAUR)
	// Wait for results
	for i := 0; i < 2; i++ {
		select {
		case u := <-chPac:
			if u.err != nil {
				err = fmt.Errorf("pacman failed: %v", u.err)
			} else {
				updates = append(updates, u.upd...)
			}
		case u := <-chAUR:
			if u.err != nil {
				err = fmt.Errorf("AUR failed: %v", u.err)
			} else {
				updates = append(updates, u.upd...)
			}
		}
	}
	return
}

func parsePacmanCheckUpdates(out string, re *regexp.Regexp, repo string) api.UpdatesList {
	updates := make(api.UpdatesList, 0)
	for _, m := range re.FindAllStringSubmatch(out, -1) {
		updates = append(updates, api.Update{
			Pkg:    m[1],
			OldVer: m[2],
			NewVer: m[3],
			Repo:   repo,
		})
	}
	return updates
}

// checkPacmanLogs read pacman log file and update internal cache accordingly
func checkPacmanLogs(fp string) error {
	file, err := os.Open(fp)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	lastChecked, err := time.Parse(time.RFC3339, cache.f.Checked)
	if err != nil {
		return fmt.Errorf("cannot parse cached time '%s': %v", cache.f.Checked, err)
	}
	beforeLen := len(cache.f.Updates)
	for scanner.Scan() {
		m := rePacmanLog.FindStringSubmatch(scanner.Text())
		if len(m) != 5 {
			continue
		}
		timestamp, action, name, ver := m[1], m[2], m[3], m[4]
		t, err := time.Parse(pacmanTimeFmt, timestamp)
		if err != nil {
			log.Debugf("checkPacmanLogs: cannot parse '%s': %v", timestamp, err)
			continue
		}
		if t.Before(lastChecked) {
			log.Debugf("checkPacmanLogs: skip '%s', timestamp too early %v", name, t)
			continue
		}
		switch action {
		case "installed":
			log.Debugf("checkPacmanLogs: skip '%s', action installed", name)
			continue
		case "upgraded":
			tmp := strings.Split(ver, " -> ")
			if len(tmp) != 2 {
				log.Warnf("checkPacmanLogs: expected 'old -> new', got '%s'", ver)
				continue
			}
			if changed := cache.f.Remove(name, tmp[1]); changed {
				log.Debugf("checkPacmanLogs: removed upgraded package %s %s", name, tmp[1])
			} else {
				log.Debugf("checkPacmanLogs: skip upgraded package %s %s", name, tmp[1])
			}
		case "removed":
			if changed := cache.f.Remove(name, ""); changed {
				log.Debugf("checkPacmanLogs: removed uninstalled package %s %s", name, ver)
			} else {
				log.Debugf("checkPacmanLogs: skip uninstalled package %s %s", name, ver)
			}
		}
	}
	if len(cache.f.Updates) != beforeLen {
		log.Infof("checkPacmanLogs: %s: removed %d pending updates", fp, beforeLen-len(cache.f.Updates))
	}
	return scanner.Err()
}
