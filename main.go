package main

import (
	"os"
	"log"
	"bufio"
	"regexp"
	"fmt"
	"flag"
	"time"
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"github.com/cosandr/go-check-updates/redhat"
	"github.com/cosandr/go-check-updates/arch"
	"github.com/cosandr/go-check-updates/types"
)

var defaultCache string = "/tmp/go-check-updates.yaml"
var defaultWait string = "24h"

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

func openHomeCache(fp *string) (file *os.File, err error) {
	log.Println("Falling back to home directory")
	usrCache, err := os.UserCacheDir()
	if err != nil {
		return
	}
	*fp = usrCache + "/go-check-updates"
	// Create if missing
	err = os.MkdirAll(*fp, 0700)
	if err != nil {
		return
	}
	*fp += "/cache.yaml"
	file, err = os.OpenFile(*fp, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	return
}

// getCacheFile returns `os.File` pointer for cache file
//
// By default it is at `defaultCache` but it may be `$HOME/.cache/go-check-updates/cache.yaml` if default is not writable.
func getCacheFile(fp *string) (file *os.File, err error) {
	file, err = os.OpenFile(*fp, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		// Retry write to user directory
		if os.IsPermission(err) {
			newFile, newErr := openHomeCache(fp)
			// Pass through errors
			err = newErr
			if newErr != nil {
				return
			}
			file = newFile
		} else {
			return
		}
	}
	log.Printf("Opened cache file: %s\n", *fp)
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
	var everyDefault, _ = time.ParseDuration(defaultWait)

	flag.StringVar(&cache, "cache", defaultCache, "Path to update cache file")
	flag.DurationVar(&updateEvery, "every", everyDefault, "How often to update cache")
	flag.Parse()

	cacheFile, err := getCacheFile(&cache)
	if err != nil {
		log.Panicln(err)
	}
	defer cacheFile.Close()

	if !needsUpdate(cacheFile, updateEvery) {
		fmt.Println("No update required")
		os.Exit(0)
	}

	var updates []types.Update
	switch distro {
	// TODO: Add other RedHat distros (RHEL, CentOS)
	case "fedora":
		updates, err = redhat.Update()
	case "arch":
		updates, err = arch.Update()
	default:
		panic(fmt.Errorf("Unsupported distro: %s", distro))
	}
	if err != nil {
		fmt.Printf("WARNING: %s\n", err)
	}
	fmt.Printf("%d updates found\n", len(updates))
	err = saveYaml(cacheFile, updates)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Cache file %s updated\n", cache)
}
