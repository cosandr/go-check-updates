package main

/*
$ yum -e0 -d0 check-update
OR
$ dnf -e0 -d0 check-update

pgdg-fedora-repo.noarch                                                42.0-6                                                 pgdg12
pgdg-fedora-repo.noarch                                                42.0-6                                                 pgdg11
pgdg-fedora-repo.noarch                                                42.0-6                                                 pgdg10
pgdg-fedora-repo.noarch                                                42.0-6                                                 pgdg96
pgdg-fedora-repo.noarch                                                42.0-6                                                 pgdg95
pgdg-fedora-repo.noarch                                                42.0-6                                                 pgdg94

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

const dnfTimeFmt = "2006-01-02T15:04:05-0700"
const oldDnfTimeFmt = "2006-01-02T15:04:05Z0700"

// Group 1: name (without arch)
// Group 2: version
// Group 3: repo
var reYum = regexp.MustCompile(`(?m)^\s*(\S+)(?:\.\S+)\s+(\S+)\s+(\S+)\s*$`)

// Group 1: timestamp
// Group 2: action (Installed, Upgrade, Upgraded, Erase)
// Group 3: <package>-<version>.<os>.<arch>
var reDnfLog = regexp.MustCompile(`^(\S+)\s+SUBDEBUG\s+(\w+):\s+(\S+)$`)

func runYum(name string) (retStr string, err error) {
	retStr, err = runCmd(name, "-e0", "-d0", "check-update")
	if err != nil {
		// DNF returns code 100 if there are updates
		if exitError, ok := err.(*exec.ExitError); ok {
			switch exitError.ExitCode() {
			case 0, 100:
				err = nil
			case 1:
				return
			}
		}
	}
	return
}

// UpdateDnf uses dnf or yum to get available updates
func UpdateDnf() (api.UpdatesList, error) {
	rawOut, err := runYum("dnf")
	// Try yum instead
	if err != nil {
		rawOut, err = runYum("yum")
	}
	// Both failed
	if err != nil {
		return api.UpdatesList{}, err
	}
	return parseYumCheckUpdate(rawOut), nil
}

func parseYumCheckUpdate(out string) api.UpdatesList {
	updates := make(api.UpdatesList, 0)
	if i := strings.Index(out, "Obsoleting Packages"); i > 0 {
		out = out[:i]
	}
	for _, m := range reYum.FindAllStringSubmatch(out, -1) {
		updates = append(updates, api.Update{
			Pkg:    m[1],
			NewVer: m[2],
			Repo:   m[3],
		})
	}
	return updates
}

// checkDnfLogs read dnf.rpm log file and update internal cache accordingly
func checkDnfLogs(fp string) error {
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
		m := reDnfLog.FindStringSubmatch(scanner.Text())
		if len(m) != 4 {
			continue
		}
		timestamp, action, name := m[1], m[2], m[3]
		t, err := time.Parse(dnfTimeFmt, timestamp)
		if err != nil {
			t, err = time.Parse(oldDnfTimeFmt, timestamp)
			if err != nil {
				log.Debugf("cannot parse '%s': %v", timestamp, err)
				continue
			}
		}
		if t.Before(lastChecked) {
			log.Debugf("skip '%s', timestamp too early %v", name, t)
			continue
		}
		switch action {
		case "Installed":
			log.Debugf("skip '%s', action installed", name)
			continue
		case "Upgrade": // Upgraded shows the old version
			if changed := cache.f.RemoveContains(name, true); changed {
				log.Debugf("removed upgraded package %s", name)
			} else {
				log.Debugf("skip upgraded package %s", name)
			}
		case "Erase":
			if changed := cache.f.RemoveContains(name, false); changed {
				log.Debugf("removed uninstalled package %s", name)
			} else {
				log.Debugf("skip uninstalled package %s", name)
			}
		}
	}
	if len(cache.f.Updates) != beforeLen {
		log.Infof("%s: removed %d pending updates", fp, beforeLen-len(cache.f.Updates))
	}
	return scanner.Err()
}
