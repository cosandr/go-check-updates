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
	"os/exec"
	"regexp"

	"github.com/cosandr/go-check-updates/api"
)

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
func UpdateDnf() (updates []api.Update, err error) {
	var rawOut string
	rawOut, err = runYum("dnf")
	// Try yum instead
	if err != nil {
		rawOut, err = runYum("yum")
	}
	// Both failed
	if err != nil {
		return
	}
	re := regexp.MustCompile(`(?m)^\s*(?P<pkg>\S+)\s+(?P<repo>\S+)\s+(?P<ver>\S+)\s*$`)
	for _, m := range re.FindAllStringSubmatch(rawOut, -1) {
		var u api.Update
		u.Pkg = m[1]
		u.NewVer = m[2]
		u.Repo = m[3]
		updates = append(updates, u)
	}
	return
}
