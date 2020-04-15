package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/cosandr/go-check-updates/types"
	"gopkg.in/yaml.v2"
)

var defaultCache string = "/tmp/go-check-updates.yaml"
var defaultWait string = "24h"
var defaultLog string = "STDOUT"

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
	return distro, fmt.Errorf("Cannot get distro ID")
}

// userCacheFallback returns user cache directory with subfolder for this program
//
// Will fail if this directory cannot be created, typically `$HOME/.cache/go-check-updates`
func userCacheFallback() (path string, err error) {
	usrCache, err := os.UserCacheDir()
	if err != nil {
		return
	}
	path = usrCache + "/go-check-updates"
	// Create if missing
	err = os.MkdirAll(path, 0700)
	if err != nil {
		return
	}
	return
}

func openHomeCache(fp *string) (file *os.File, err error) {
	*fp, err = userCacheFallback()
	if err != nil {
		return
	}
	*fp += "/cache.yaml"
	file, err = os.OpenFile(*fp, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return
	}
	return
}

func openHomeLog(fp *string) (file *os.File, err error) {
	*fp, err = userCacheFallback()
	if err != nil {
		return
	}
	*fp += "/log"
	file, err = os.OpenFile(*fp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	return
}

func getLogFile(fp *string) (file *os.File, err error) {
	file, err = os.OpenFile(*fp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
		// Retry write to user directory
		newFile, newErr := openHomeLog(fp)
		// Pass through errors
		err = newErr
		if newErr != nil {
			return
		}
		file = newFile
	}
	return
}

// getCacheFile returns `os.File` pointer for cache file
//
// By default it is at `defaultCache` but it may be `$HOME/.cache/go-check-updates/cache.yaml` if default is not writable.
func getCacheFile(fp *string) (file *os.File, err error) {
	file, err = os.OpenFile(*fp, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Println(err)
		// Retry write to user directory
		newFile, newErr := openHomeCache(fp)
		// Pass through errors
		err = newErr
		if newErr != nil {
			return
		}
		file = newFile
	}
	return
}

func needsUpdate(file *os.File, dur time.Duration) bool {
	var yml types.YamlT
	err := readYaml(&yml, file)
	// No cache, needs update
	if err != nil {
		return true
	}
	log.Printf("Cache last update: %s\n", yml.Checked.String())
	lastUpdate := time.Since(yml.Checked)
	if lastUpdate.Seconds() > dur.Seconds() {
		return true
	}
	return false
}

func readYaml(y *types.YamlT, file *os.File) (err error) {
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return
	}
	err = yaml.Unmarshal(bytes, y)
	if err != nil {
		return
	}
	return
}

func saveYaml(file *os.File, updates []types.Update) (err error) {
	var yml types.YamlT
	yml.Updates = updates
	yml.Checked = time.Now()
	bytes, err := yaml.Marshal(&yml)
	if err != nil {
		return
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return
	}
	err = file.Truncate(0)
	if err != nil {
		return
	}
	_, err = file.Write(bytes)
	if err != nil {
		return
	}
	return
}

func main() {
	distro, err := getDistro()
	if err != nil {
		log.Panicln(err)
	}
	var cache string
	var updateEvery time.Duration
	var noLogging bool
	var logFileFp string
	var everyDefault, _ = time.ParseDuration(defaultWait)

	flag.StringVar(&cache, "cache", defaultCache, "Path to update cache file")
	flag.DurationVar(&updateEvery, "every", everyDefault, "How often to update cache")
	flag.BoolVar(&noLogging, "nolog", false, "Disable logging")
	flag.StringVar(&logFileFp, "logfile", defaultLog, "Path to log file")
	flag.Parse()

	if noLogging {
		fmt.Println("Logging disabled")
		log.SetOutput(ioutil.Discard)
	} else if logFileFp != "STDOUT" {
		file, err := getLogFile(&logFileFp)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		fmt.Printf("Saving log to: %s\n", logFileFp)
		log.SetOutput(file)
	}

	cacheFile, err := getCacheFile(&cache)
	if err != nil {
		log.Panicln(err)
	}
	defer cacheFile.Close()
	log.Printf("Opened cache file: %s\n", cache)

	if !needsUpdate(cacheFile, updateEvery) {
		log.Println("No update required")
		return
	}

	var updates []types.Update
	switch distro {
	// TODO: Add other RedHat distros (RHEL, CentOS)
	case "fedora":
		updates, err = UpdateDnf()
	case "arch":
		updates, err = UpdateArch()
	default:
		log.Panicf("Unsupported distro: %s\n", distro)
	}
	if err != nil {
		log.Printf("WARNING: %s\n", err)
	}
	log.Printf("%d updates found\n", len(updates))
	err = saveYaml(cacheFile, updates)
	if err != nil {
		log.Panicln(err)
	}
	log.Printf("Cache file %s updated\n", cache)
}
