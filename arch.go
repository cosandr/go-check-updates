package main

/*
$ checkupdates
libarchive 3.4.0-3 -> 3.4.1-1
libjpeg-turbo 2.0.3-1 -> 2.0.4-1
linux 5.4.6.arch3-1 -> 5.4.7.arch1-1
linux-headers 5.4.6.arch3-1 -> 5.4.7.arch1-1
shellcheck 0.7.0-82 -> 0.7.0-83
##########
$ checkupdates && pikaur -Qua 2>/dev/null
libarchive 3.4.0-3 -> 3.4.1-1
libjpeg-turbo 2.0.3-1 -> 2.0.4-1
linux 5.4.6.arch3-1 -> 5.4.7.arch1-1
linux-headers 5.4.6.arch3-1 -> 5.4.7.arch1-1
shellcheck 0.7.0-82 -> 0.7.0-83
 corefreq-git                          1.70-1               -> 1.71-1
 pikaur                                1.5.7-1              -> 1.5.8-1
*/

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/cosandr/go-check-updates/types"
)

func checkPikaur() (ret string, err error) {
	var outBuf bytes.Buffer
	cmd := exec.Command("pikaur", "-Qua")
	cmd.Stdout = &outBuf
	err = cmd.Run()
	if err != nil {
		return
	}
	ret = outBuf.String()
	return
}

func checkPacman() (ret string, err error) {
	var outBuf bytes.Buffer
	cmd := exec.Command("checkupdates")
	cmd.Stdout = &outBuf
	err = cmd.Run()
	if err != nil {
		return
	}
	ret = outBuf.String()
	return
}

func procPacman(updates *[]types.Update, wg *sync.WaitGroup, err *error) {
	raw, errPac := checkPacman()
	if errPac != nil {
		*err = errPac
		wg.Done()
		return
	}
	// Strip newlines at the end
	raw = strings.TrimSuffix(raw, "\n")
	var reSpaces = regexp.MustCompile(`\s+`)
	// Loop through each pending update
	for _, line := range strings.Split(raw, "\n") {
		// shellcheck 0.7.0-82 -> 0.7.0-83
		// Split into package | oldver | -> | newver
		noSpaces := reSpaces.ReplaceAllString(line, "\t")
		data := strings.Split(noSpaces, "\t")
		// Skip invalid data
		if len(data) < 4 {
			continue
		}
		var u types.Update
		u.Pkg = data[0]
		u.OldVer = data[1]
		u.NewVer = data[3]
		u.Repo = "pacman"
		*updates = append(*updates, u)
	}
	wg.Done()
}

func procPikaur(updates *[]types.Update, wg *sync.WaitGroup, err *error) {
	raw, errPik := checkPikaur()
	if errPik != nil {
		*err = errPik
		wg.Done()
		return
	}
	// Strip newlines at the end
	raw = strings.TrimSuffix(raw, "\n")
	var reStart = regexp.MustCompile(`^\s+`)
	var reSpaces = regexp.MustCompile(`\s+`)
	// Loop through each pending update
	for _, line := range strings.Split(raw, "\n") {
		// Strip leading spaces
		line = reStart.ReplaceAllString(line, "")
		// shellcheck 0.7.0-82 -> 0.7.0-83
		// Split into package | oldver | -> | newver
		noSpaces := reSpaces.ReplaceAllString(line, "\t")
		data := strings.Split(noSpaces, "\t")
		// Skip invalid data
		if len(data) < 4 {
			continue
		}
		var u types.Update
		u.Pkg = data[0]
		u.OldVer = data[1]
		u.NewVer = data[3]
		u.Repo = "aur"
		*updates = append(*updates, u)
	}
	wg.Done()
}

type updRes struct {
	upd []types.Update
	err error
}

// UpdateArch uses checkupdates and (if available) pikaur to get available updates
func UpdateArch() (updates []types.Update, err error) {
	var wg sync.WaitGroup
	var pacUpd updRes
	var aurUpd updRes
	// Run in parallel
	wg.Add(1)
	go procPacman(&pacUpd.upd, &wg, &pacUpd.err)
	wg.Add(1)
	go procPikaur(&aurUpd.upd, &wg, &aurUpd.err)
	wg.Wait()
	// Both failed
	if pacUpd.err != nil && aurUpd.err != nil {
		err = fmt.Errorf("Pacman: %s\nPikaur: %s", pacUpd.err, aurUpd.err)
		return
	}
	// Concatenate and check for errors
	for n, u := range map[string]updRes{"pacman": pacUpd, "aur": aurUpd} {
		if u.err != nil {
			err = fmt.Errorf("%s: %s", n, u.err)
			continue
		}
		updates = append(updates, u.upd...)
	}
	return
}
