package redhat

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
	"strings"
	"bytes"
	"regexp"
	"github.com/cosandr/go-check-updates/types"
)

func runCmd(name string, buf *bytes.Buffer) (retStr string, err error) {
	cmd := exec.Command(name, "-e0", "-d0", "check-update")
	cmd.Stdout = buf
	err = cmd.Run()
	retStr = buf.String()
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
	buf.Reset()
	return
}

// Update uses dnf or yum to get available updates
func Update() (updates []types.Update, err error) {
	var buf bytes.Buffer
	var rawOut string
	rawOut, err = runCmd("dnf", &buf)
	// Try yum instead
	if err != nil {
		rawOut, err = runCmd("yum", &buf)
	}
	// Both failed
	if err != nil {
		return
	}
	// Strip newlines at start and end
	rawOut = strings.TrimPrefix(rawOut, "\n")
	rawOut = strings.TrimSuffix(rawOut, "\n")
	var reSpaces = regexp.MustCompile(`\s+`)
	// Loop through each pending update
	for _, line := range strings.Split(rawOut, "\n") {
		// Split into package, version, repo
		noSpaces := reSpaces.ReplaceAllString(line, "\t")
		data := strings.Split(noSpaces, "\t")
		// Check for `Obsoleting Packages`
		if len(data) > 1 {
			if data[0] == "Obsoleting" && data[1] == "Packages" {
				break
			}
		}
		// Skip invalid data
		if len(data) < 3 {
			continue
		}
		var u types.Update
		u.Pkg = data[0]
		u.NewVer = data[1]
		u.Repo = data[2]
		updates = append(updates, u)
	}
	return
}
