package main

import (
	"flag"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/cosandr/go-check-updates/api"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/writer"
)

const (
	packageName string = "go-check-updates"
	defaultWait string = "12h"
)

var (
	distro   string
	upgrader websocket.Upgrader
	cache    InternalCache
)

func init() {
	ret, err := getDistro()
	if err != nil {
		log.Panicln(err)
	}
	distro = ret
	upgrader = websocket.Upgrader{}
}

func main() {
	var (
		argDaemon          bool
		argDebug           bool
		argQuiet           bool
		argSystemd         bool
		argCacheInterval   time.Duration
		argCachePath       string
		argListenAddress   string
		argLogFile         string
		cacheFp            string
		defaultInterval, _ = time.ParseDuration(defaultWait)
		defaultCache, _    = getCachePath()
		err                error
		listener           net.Listener
	)

	flag.BoolVar(&argDaemon, "daemon", false, "Run HTTP server as a daemon")
	flag.BoolVar(&argDebug, "debug", false, "Set console log output to DEBUG")
	flag.BoolVar(&argQuiet, "q", false, "Don't log to console")
	flag.BoolVar(&argSystemd, "systemd", false, "Run HTTP server using systemd socket activation")
	flag.DurationVar(&argCacheInterval, "cache.interval", defaultInterval, "Time interval between cache updates")
	flag.StringVar(&argCachePath, "cache.path", defaultCache, "Path to update cache file, empty string to disable")
	flag.StringVar(&argListenAddress, "web.listen-address", ":8100", "Web server listen address")
	flag.StringVar(&argLogFile, "log.file", "", "Path to log file")
	flag.Parse()
	//
	// Logging setup
	//
	if argSystemd || argDaemon {
		// Disable timestamps when running in background mode
		// They are not needed as these modes are most likely used with systemd
		// which adds its own timestamps
		log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})
	} else {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	}
	log.SetOutput(ioutil.Discard)
	levels := []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
	}
	if argDebug {
		log.SetLevel(log.DebugLevel)
		levels = append(levels, log.DebugLevel)
	}
	if !argQuiet {
		log.AddHook(&writer.Hook{
			Writer:    os.Stderr,
			LogLevels: levels,
		})
	}
	if argLogFile != "" {
		file, err := os.OpenFile(argLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		log.AddHook(&writer.Hook{
			Writer:    file,
			LogLevels: levels,
		})
	}
	//
	// Logging setup
	//
	if argCachePath == "" {
		cacheFp = ""
		log.Info("No cache file")
	} else if argCachePath != defaultCache {
		cacheFp = argCachePath
		if checkFileRead(cacheFp) && !checkFileWrite(cacheFp) {
			log.Fatal("cache file is not writable")
		}
		log.Infof("Using provided cache file: %s", cacheFp)
	} else {
		cacheFp, err = getCachePath()
		if err != nil {
			log.Fatalf("No suitable cache file: %v", err)
		}
		log.Infof("Using auto path for cache file: %s", cacheFp)
	}
	cache = InternalCache{
		Cond: sync.NewCond(&sync.Mutex{}),
		f:    api.File{},
		fp:   cacheFp,
	}
	if argSystemd {
		listeners, err := activation.Listeners()
		if err != nil {
			log.Panic(err)
		}

		if len(listeners) != 1 {
			log.Panic("Unexpected number of socket activation fds")
		}
		listener = listeners[0]
	} else if argDaemon {
		listener, err = net.Listen("tcp", argListenAddress)
		if err != nil {
			log.Panicf("Cannot listen: %s", err)
		}
	}

	if argSystemd || argDaemon {
		if argCacheInterval.Seconds() > 0 {
			go func() {
				ticker := time.NewTicker(argCacheInterval)
				for {
					select {
					case <-ticker.C:
						log.Debug("autoUpdate: refresh")
						cache.Update()
					}
				}
			}()
		}
		http.HandleFunc("/api", HandleAPI)
		http.HandleFunc("/ws", HandleWS)
		log.Infof("Listening on %s", listener.Addr().String())
		err = http.Serve(listener, nil)
		if err != nil {
			log.Errorf("HTTP serve error: %v", err)
			os.Exit(2)
		}
		os.Exit(0)
	}

	// No HTTP stuff, just run normally
	if !cache.NeedsUpdate(argCacheInterval) {
		log.Info("No update required")
		return
	}

	cache.Update()

}