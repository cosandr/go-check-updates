package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"time"

	"github.com/cosandr/go-check-updates/api"
	log "github.com/sirupsen/logrus"
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
	re := regexp.MustCompile(`ID=(.*)`)
	for scanner.Scan() {
		m := re.FindSubmatch(scanner.Bytes())
		if len(m) > 1 {
			distro = string(m[1])
			return
		}
	}
	return distro, fmt.Errorf("cannot get distro ID")
}

// userHomeFallback returns user cache directory with subfolder for this program
//
// Will fail if this directory cannot be created, typically `$HOME/.cache/go-check-updates`
func userHomeFallback() (path string, err error) {
	usrCache, err := os.UserCacheDir()
	if err != nil {
		return
	}
	path = usrCache + "/go-check-updates"
	// Create if missing
	err = os.MkdirAll(path, 0700)
	return
}

// getHomeCache returns a file pointer to the cache file in the user's cache directory
func getHomeCache() (string, error) {
	fp, err := userHomeFallback()
	if err != nil {
		return "", err
	}
	fp += "/cache.json"
	file, err := os.OpenFile(fp, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return file.Name(), nil
}

// getCachePath returns path to cache file
//
// By default it is at `defaultCache` but it may be `$HOME/.cache/go-check-updates/cache.json` if default is not writable.
func getCachePath() (string, error) {
	file, err := os.OpenFile(defaultCache, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		// Retry write to user directory
		return getHomeCache()
	}
	defer file.Close()
	return file.Name(), nil
}

// openHomeLog returns a file pointer to the log file in the user's cache directory
func openHomeLog() (file *os.File, err error) {
	fp, err := userHomeFallback()
	if err != nil {
		return
	}
	fp += "/log"
	return os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
}

// getLogFile tries to open the requested path, if it fails, a cache file in the user's cache directory is used
func getLogFile(fp string) (file *os.File, err error) {
	file, err = os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		// Retry write to user directory
		return openHomeLog()
	}
	return
}

// needsUpdate attempts to read the cache file and determines if it needs an update
//
// Malformed files are considered invalid and will be replaced
func needsUpdate(dur time.Duration) bool {
	f, err := readCacheFile()
	// Cannot read, update
	if err != nil {
		return true
	}
	log.Infof("needsUpdate: last update: %s", f.Checked)
	t, err := time.Parse(time.RFC3339, f.Checked)
	// Can't parse timestamp, update
	if err != nil {
		return true
	}
	lastUpdate := time.Since(t)
	if lastUpdate.Seconds() > dur.Seconds() {
		return true
	}
	return false
}

// checkFileRW returns true if the file can be written to OK
// It is considered invalid if the current user is not its owner
func checkFileRW(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

func readCacheFile() (yml api.File, err error) {
	bytes, err := ioutil.ReadFile(cacheFilePath)
	if err != nil {
		return
	}
	err = json.Unmarshal(bytes, &yml)
	return
}

func updateLatestFile(f *api.File) {
	latestFile.L.Lock()
	log.Debug("updateLatestFile: lock acquired")
	latestFile.f = f.Copy()
	latestFile.Broadcast()
	log.Debug("updateLatestFile: broadcast")
	latestFile.L.Unlock()
	log.Debug("updateLatestFile: unlock")
}

func writeCacheFile(updates []api.Update) (err error) {
	var f api.File
	f.Updates = updates
	f.Checked = time.Now().Format(time.RFC3339)
	updateLatestFile(&f)
	log.Debug("writeCacheFile: marshal file")
	bytes, err := json.Marshal(&f)
	if err != nil {
		return
	}
	log.Debugf("writeCacheFile: write file %s", cacheFilePath)
	err = ioutil.WriteFile(cacheFilePath, bytes, 0644)
	return
}
