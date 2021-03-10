package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// getDistro returns the host OS' distro ID (e.g. 'fedora', 'arch')
func getDistro() (distro string, err error) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Look for pretty name
	re := regexp.MustCompile(`ID="?([^"\s]+)"?`)
	for scanner.Scan() {
		m := re.FindSubmatch(scanner.Bytes())
		if len(m) > 1 {
			distro = string(m[1])
			return
		}
	}
	return distro, fmt.Errorf("cannot get distro ID")
}

// userHomeCacheDir returns user cache directory with subfolder for this program
//
// Will fail if this directory cannot be created, typically `$HOME/.cache/go-check-updates`
func userHomeCacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	ret := path.Join(dir, packageName)
	// Create if missing
	err = os.MkdirAll(ret, 0700)
	return ret, err
}

// getHomeCache returns a file pointer to the cache file in the user's cache directory
func getHomeCache() (string, error) {
	dir, err := userHomeCacheDir()
	if err != nil {
		return "", err
	}
	return path.Join(dir, "cache.json"), nil
}

// getCachePath returns path to cache file
//
// By default it is at `defaultCache` but it may be `$HOME/.cache/go-check-updates/cache.json` if default is not writable.
func getCachePath() (string, error) {
	dir := os.TempDir()
	tmpCache := path.Join(dir, packageName+".json")
	if checkFileExists(tmpCache) && !checkFileWrite(tmpCache) {
		return getHomeCache()
	}
	return tmpCache, nil
}

// checkFileWrite returns nil if the file can be written to OK
// It is considered invalid if the current user is not its owner
func checkFileWrite(path string) bool {
	return unix.Access(path, unix.W_OK|unix.R_OK) == nil
}

// checkFileRead returns true if the file is readable
func checkFileRead(path string) bool {
	return unix.Access(path, unix.R_OK) == nil
}

// checkFileExists returns true if the file exists
func checkFileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func runCmd(name string, args ...string) (string, error) {
	log.Debugf("runCmd %s %s", name, args)
	var buf bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &buf
	err := cmd.Run()
	return buf.String(), err
}

// checkCmd returns true if '<name> --help' ran successfully
func checkCmd(name string) bool {
	cmd := exec.Command(name, "--help")
	err := cmd.Run()
	return err == nil
}
