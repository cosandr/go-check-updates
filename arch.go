package main

/*
$ checkupdates
libarchive 3.4.0-3 -> 3.4.1-1
libjpeg-turbo 2.0.3-1 -> 2.0.4-1
linux 5.4.6.arch3-1 -> 5.4.7.arch1-1
linux-headers 5.4.6.arch3-1 -> 5.4.7.arch1-1
shellcheck 0.7.0-82 -> 0.7.0-83
##########
$ pikaur -Qua 2>/dev/null
 corefreq-git                          1.70-1               -> 1.71-1
 pikaur                                1.5.7-1              -> 1.5.8-1
##########
/var/log/pacman.log
[2020-05-29T23:47:13+0200] [ALPM] upgraded linux-firmware (20200421.78c0348-1 -> 20200519.8ba6fa6-1)
[2020-05-29T23:47:18+0200] [ALPM] upgraded linux-headers (5.6.14.arch1-1 -> 5.6.15.arch1-1)
[2020-05-29T23:47:18+0200] [ALPM] upgraded networkmanager (1.24.0-1 -> 1.24.2-1)
[2020-05-29T23:47:18+0200] [ALPM] upgraded python-setuptools (1:47.1.0-1 -> 1:47.1.1-1)
[2020-05-29T23:47:18+0200] [ALPM] upgraded sbsigntools (0.9.3-1 -> 0.9.3-2)
[2020-05-29T23:47:18+0200] [ALPM] upgraded shellcheck (0.7.1-32 -> 0.7.1-33)
[2020-05-29T23:47:18+0200] [ALPM] upgraded vte-common (0.60.2-2 -> 0.60.3-1)
[2020-05-29T23:47:18+0200] [ALPM] upgraded vte3 (0.60.2-2 -> 0.60.3-1)
[2020-05-29T23:47:11+0200] [ALPM] installed haskell-these (1.1-1)
[2020-05-12T10:19:08+0200] [ALPM] removed ovmf (1:202002-1)
[2020-05-12T10:19:08+0200] [ALPM] removed libwbclient (4.11.3-3)
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
	name string
	args string
	// Optional regex pattern for updates, defaults to pacman regex if nil
	re *regexp.Regexp
}

type updRes struct {
	upd []api.Update
	err error
}

const pacmanTimeFmt = "2006-01-02T15:04:05-0700" // old format "2006-01-02 15:04"

var rePacman = regexp.MustCompile(`(?m)^\s*(?P<pkg>\S+)\s+(?P<oldver>\S+)\s+->\s+(?P<newver>\S+)\s*$`)

// Group 1: timestamp
// Group 2: action (installed, upgraded, removed)
// Group 3: package
// Group 4: if installed or removed, version
//			if upgraded, <oldVersion> -> <newVersion> (same as pacman -Qu)
var rePacmanLog = regexp.MustCompile(`^\[(\S+)\]\s\[ALPM\]\s(\w+)\s(\S+)\s\((.*)\)$`)

var supportedHelpers = []helper{
	{
		name: "yay",
		args: "-Qua",
		re:   rePacman,
	},
	{
		name: "paru",
		args: "-Qua",
		re:   rePacman,
	},
	{
		name: "pikaur",
		args: "-Qua",
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

	for _, m := range rePacman.FindAllStringSubmatch(raw, -1) {
		var u api.Update
		u.Pkg = m[1]
		u.OldVer = m[2]
		u.NewVer = m[3]
		u.Repo = "pacman"
		res.upd = append(res.upd, u)
	}
}

func procAUR(ch chan<- updRes) {
	res := updRes{}
	defer func() {
		ch <- res
	}()
	if aur.name == "" {
		log.Debug("no AUR helper, skipping")
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
	for _, m := range aur.re.FindAllStringSubmatch(raw, -1) {
		var u api.Update
		u.Pkg = m[1]
		u.OldVer = m[2]
		u.NewVer = m[3]
		u.Repo = "aur"
		res.upd = append(res.upd, u)
	}
}

// UpdateArch uses checkupdates and (if available) a supported AUR helper to get available updates
func UpdateArch() (updates []api.Update, err error) {
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
			log.Debugf("cannot parse '%s': %v", timestamp, err)
			continue
		}
		if t.Before(lastChecked) {
			log.Debugf("skip '%s', timestamp too early %v", name, t)
			continue
		}
		switch action {
		case "installed":
			log.Debugf("skip '%s', action installed", name)
			continue
		case "upgraded":
			tmp := strings.Split(ver, " -> ")
			if len(tmp) != 2 {
				log.Warnf("expected 'old -> new', got '%s'", ver)
				continue
			}
			if changed := cache.f.RemoveIfNew(name, tmp[1]); changed {
				log.Debugf("removed upgraded package %s %s", name, tmp[1])
			} else {
				log.Debugf("skip upgraded package %s %s", name, tmp[1])
			}
		case "removed":
			if changed := cache.f.RemoveIfNew(name, ver); changed {
				log.Debugf("removed uninstalled package %s %s", name, ver)
			} else {
				log.Debugf("skip uninstalled package %s %s", name, ver)
			}
		}
	}
	if len(cache.f.Updates) != beforeLen {
		log.Infof("%s: removed %d pending updates", fp, beforeLen-len(cache.f.Updates))
	}
	return scanner.Err()
}
