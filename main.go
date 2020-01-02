package main

import (
	"os"
	"bufio"
	"regexp"
	"fmt"
	"flag"
	"time"
	"github.com/cosandr/go-check-updates/redhat"
	"github.com/cosandr/go-check-updates/arch"
	"github.com/cosandr/go-check-updates/types"
	"gopkg.in/yaml.v2"
)

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

func needsUpdate(fp string, dur time.Duration) bool {
	file, err := os.Open(fp)
	// No cache, needs update
	if err != nil {
		return true
	}
	defer file.Close()
	stats, err := file.Stat()
	// Something is wrong with file, try to update
	if err != nil {
		return true
	}
	lastUpdate := time.Since(stats.ModTime())
	if lastUpdate.Seconds() > dur.Seconds() {
		return true
	}
	return false
}

func writeBytes(bytes []byte, fp string) (err error) {
	file, err := os.Create(fp)
	if err != nil {
		return
	}
	defer file.Close()
	file.Write(bytes)
	file.Chmod(0644)
	return
}

func saveYaml(fp string, updates []types.Update) (err error) {
	var yml types.YamlT
	yml.Updates = updates
	yml.Checked = time.Now()
	bytes, err := yaml.Marshal(&yml)
	if err != nil {
		return
	}
	err = writeBytes(bytes, fp)
	if err != nil {
		return
	}
	return
}

func main() {
	distro, err := getDistro()
	if err != nil {
		panic(err)
	}
	var cache string
	var updateEvery time.Duration
	var everyDefault, _ = time.ParseDuration("24h")

	flag.StringVar(&cache, "cache", "/tmp/go-updates.yaml", "Path to update cache file")
	flag.DurationVar(&updateEvery, "every", everyDefault, "How often to update cache")
	flag.Parse()

	if !needsUpdate(cache, updateEvery) {
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
	if len(updates) == 0 {
		fmt.Println("No updates found")
		os.Exit(0)
	}
	err = saveYaml(cache, updates)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Cache file %s updated\n", cache)
}
