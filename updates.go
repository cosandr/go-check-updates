package main

import (
	"flag"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/cosandr/go-check-updates/api"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// LatestFile holds a threadsafe api.File
type LatestFile struct {
	*sync.Cond
	f api.File
}

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
	globalWg      sync.WaitGroup
	upgrader      = websocket.Upgrader{}
	latestFile    = LatestFile{sync.NewCond(&sync.Mutex{}), api.File{}}
)

// updateCache runs the distro specific update function and writes it to the cache file
func updateCache() (err error) {
	updates, err := runFunc()
	if err != nil {
		log.Warn(err)
	}
	log.Debugf("updateCache: %d updates found", len(updates))
	sort.Slice(updates, func(i, j int) bool {
		return updates[i].Pkg < updates[j].Pkg
	})
	err = writeCacheFile(updates)
	if err != nil {
		log.Warn(err)
		return
	}
	log.Infof("updateCache: file %s updated", cacheFilePath)
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
	var (
		daemon          bool
		everyDefault, _ = time.ParseDuration(defaultWait)
		listenAddress   string
		logFileFp       string
		quiet           bool
		reqCacheFile    string
		socketActivate  bool
		updateEvery     time.Duration
	)

	flag.BoolVar(&quiet, "q", false, "Disable logging")
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&daemon, "daemon", false, "Run REST API as a daemon")
	flag.BoolVar(&socketActivate, "systemd", false, "Run REST API using systemd socket activation")
	flag.StringVar(&reqCacheFile, "cache", defaultCache, "Path to update cache file")
	flag.StringVar(&logFileFp, "logfile", defaultLog, "Path to log file")
	flag.StringVar(&listenAddress, "web.listen-address", ":8100", "Web server listen address")
	flag.DurationVar(&updateEvery, "every", everyDefault, "How often to update cache")
	flag.Parse()

	if reqCacheFile != defaultCache {
		cacheFilePath = reqCacheFile
		if err := checkFileRW(cacheFilePath); err != nil {
			log.Fatalf("Cannot open cache file: %v", err)
		}
	} else {
		cacheFilePath, err = getCachePath()
		if err != nil {
			log.Fatalf("No suitable cache file: %v", err)
		}
	}
	// Logging setup
	// Disable timestamps when running in background mode
	// They are not needed as these modes are most likely used with systemd
	// which adds its own timestamps
	if socketActivate || daemon {
		log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})
	} else {
		log.SetFormatter(&log.TextFormatter{DisableTimestamp: false, FullTimestamp: true})
	}
	if debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug mode")
	} else {
		log.SetLevel(log.InfoLevel)
	}
	if quiet {
		log.SetOutput(ioutil.Discard)
	} else if logFileFp != "STDOUT" {
		file, err := getLogFile(logFileFp)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		log.Infof("Saving log to: %s", logFileFp)
		log.SetOutput(file)
	}
	log.Infof("Using cache file: %s", cacheFilePath)
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
			log.Panicf("Cannot listen: %s", err)
		}
	}

	if socketActivate || daemon {
		http.HandleFunc("/api", HandleAPI)
		http.HandleFunc("/ws", HandleWS)
		log.Infof("Listening on %s", listener.Addr().String())
		err = http.Serve(listener, nil)
		globalWg.Wait()
		if err != nil {
			log.Errorf("HTTP serve error: %v", err)
			os.Exit(2)
		}
		os.Exit(0)
	}

	// No HTTP stuff, just run normally
	if !needsUpdate(updateEvery) {
		log.Info("No update required")
		return
	}

	_ = updateCache()

}
