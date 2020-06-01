package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/cosandr/go-check-updates/api"
)

const (
	defaultCache string = "/tmp/go-check-updates.json"
	defaultWait  string = "24h"
	defaultLog   string = "STDOUT"
	contentType  string = "application/json"
)

var (
	// runFunc is the function that will be run to get updates
	runFunc func() ([]api.Update, error)
	// cacheFilePath is the path to cache file in use
	cacheFilePath string
	debug         bool
	wg            sync.WaitGroup
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
	return
}

// openHomeCache returns a file pointer to the cache file in the user's cache directory
func openHomeCache(fp string) (file *os.File, err error) {
	fp, err = userCacheFallback()
	if err != nil {
		return
	}
	fp += "/cache.json"
	file, err = os.OpenFile(fp, os.O_RDWR|os.O_CREATE, 0600)
	return
}

// openHomeLog returns a file pointer to the log file in the user's cache directory
func openHomeLog(fp string) (file *os.File, err error) {
	fp, err = userCacheFallback()
	if err != nil {
		return
	}
	fp += "/log"
	file, err = os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	return
}

// getLogFile tries to open the requested path, if it fails, a cache file in the user's cache directory is used
func getLogFile(fp string) (file *os.File, err error) {
	file, err = os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
		// Retry write to user directory
		file, err = openHomeLog(fp)
	}
	return
}

// getCacheFile returns `os.File` pointer for cache file
//
// By default it is at `defaultCache` but it may be `$HOME/.cache/go-check-updates/cache.json` if default is not writable.
func getCacheFile(fp string) (file *os.File, err error) {
	file, err = os.OpenFile(fp, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		if os.Geteuid() == 0 {
			// Delete file and try again
			_ = os.Remove(fp)
			file, err = os.OpenFile(fp, os.O_RDWR|os.O_CREATE, 0644)
		} else {
			// Fallback to home directory cache
			log.Println(err)
			// Retry write to user directory
			file, err = openHomeCache(fp)
		}
	}
	if err != nil && file != nil {
		cacheFilePath = file.Name()
	}
	return
}

// needsUpdate attempts to read the cache file and determines if it needs an update
//
// Malformed files are considered invalid and will be replaced
func needsUpdate(path string, dur time.Duration) bool {
	file, err := getCacheFile(path)
	// No cache, update
	if err != nil {
		return true
	}
	defer file.Close()
	f, err := readFile(file)
	// Cannot read, update
	if err != nil {
		return true
	}
	log.Printf("Cache last update: %s\n", f.Checked)
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

func readFile(file *os.File) (yml api.File, err error) {
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return
	}
	err = json.Unmarshal(bytes, &yml)
	return
}

func writeFile(writeFile *os.File, updates []api.Update) (err error) {
	var f api.File
	f.Updates = updates
	f.Checked = time.Now().Format(time.RFC3339)
	bytes, err := json.Marshal(&f)
	if err != nil {
		return
	}
	_, err = writeFile.Seek(0, 0)
	if err != nil {
		return
	}
	err = writeFile.Truncate(0)
	if err != nil {
		return
	}
	_, err = writeFile.Write(bytes)
	return
}

// updateFile runs the distro specific update function and writes it to the cache file
func updateFile() (err error) {
	updates, err := runFunc()
	if err != nil {
		log.Printf("WARNING: %s\n", err)
	}
	if debug {
		log.Printf("%d updates found\n", len(updates))
	}
	cacheFile, err := getCacheFile(cacheFilePath)
	if err != nil {
		log.Println(err)
		return
	}
	defer cacheFile.Close()
	err = writeFile(cacheFile, updates)
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("Cache file %s updated\n", cacheFile.Name())
	return
}

func main() {
	distro, err := getDistro()
	if err != nil {
		log.Panicln(err)
	}

	// TODO: Add other RedHat distros (RHEL, CentOS)
	funcMap := map[string]func() ([]api.Update, error){
		"fedora": UpdateDnf,
		"arch":   UpdateArch,
	}

	if val, ok := funcMap[distro]; ok {
		runFunc = val
	} else {
		log.Panicf("Unsupported distro: %s\n", distro)
	}

	var updateEvery time.Duration
	var quiet bool
	var daemon bool
	var listenAddress string
	var socketActivate bool
	var logFileFp string
	var everyDefault, _ = time.ParseDuration(defaultWait)

	flag.BoolVar(&quiet, "q", false, "Disable logging")
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&daemon, "daemon", false, "Run REST API as a daemon")
	flag.BoolVar(&socketActivate, "systemd", false, "Run REST API using systemd socket activation")
	flag.StringVar(&cacheFilePath, "cache", defaultCache, "Path to update cache file")
	flag.StringVar(&logFileFp, "logfile", defaultLog, "Path to log file")
	flag.StringVar(&listenAddress, "web.listen-address", ":8100", "Web server listen address")
	flag.DurationVar(&updateEvery, "every", everyDefault, "How often to update cache")
	flag.Parse()

	if quiet {
		if debug {
			fmt.Println("Logging disabled")
		}
		log.SetOutput(ioutil.Discard)
	} else if logFileFp != "STDOUT" {
		file, err := getLogFile(logFileFp)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		fmt.Printf("Saving log to: %s\n", logFileFp)
		log.SetOutput(file)
	}

	var listener net.Listener
	if socketActivate {
		listeners, err := activation.Listeners()
		if err != nil {
			log.Panic(err)
		}

		if len(listeners) != 1 {
			log.Panic("Unexpected number of socket activation fds")
		}
		listener = listeners[0]
	} else if daemon {
		listener, err = net.Listen("tcp", listenAddress)
		if err != nil {
			log.Panicf("cannot listen: %s", err)
		}
	}

	if socketActivate || daemon {
		http.HandleFunc("/cached", HandleCached)
		http.HandleFunc("/run", HandleRun)
		log.Printf("Listening on %s", listener.Addr().String())
		err = http.Serve(listener, nil)
		wg.Wait()
		if err != nil {
			log.Printf("HTTP serve error: %v", err)
			os.Exit(2)
		}
		os.Exit(0)
	}

	// No HTTP stuff, just run normally
	if !needsUpdate(cacheFilePath, updateEvery) {
		log.Println("No update required")
		return
	}

	_ = updateFile()

}
