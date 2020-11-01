package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/coreos/go-systemd/v22/activation"
	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/writer"
)

const (
	packageName string = "go-check-updates"
	defaultWait string = "12h"
)

var aur = helper{}
var cache = NewInternalCache()
var args struct {
	AurHelper     string        `arg:"--aur" help:"Override AUR helper (Arch Linux)"`
	CacheFile     string        `arg:"--cache.file,env:CACHE_FILE" help:"Path to update cache file"`
	CacheInterval time.Duration `arg:"--cache.interval,env:CACHE_INTERVAL" help:"Time interval between cache updates"`
	Daemon        bool          `arg:"-d,--daemon" help:"Run as a daemon"`
	Debug         bool          `arg:"--debug,env:DEBUG" help:"Set console log output to DEBUG"`
	ListenAddress string        `arg:"--web.listen-address,env:LISTEN_ADDRESS" help:"Web server listen address" default:":8100"`
	LogFile       string        `arg:"--log.file,env:LOG_FILE" help:"Path to log file"`
	LogLevel      string        `arg:"--log.level,env:LOG_LEVEL" default:"INFO" help:"Set log level"`
	NoCache       bool          `arg:"--no-cache,env:NO_CACHE" help:"Don't use cache file"`
	NoLogFile     bool          `arg:"--no-log,env:NO_LOG_FILE" help:"Don't log to file"`
	NoRefresh     bool          `arg:"--no-refresh,env:NO_REFRESH" help:"Don't auto-refresh"`
	Quiet         bool          `arg:"-q,--quiet" help:"Don't log to console"`
	Systemd       bool          `arg:"--systemd" help:"Run HTTP server using systemd socket activation"`
	Watch         bool          `arg:"-w,--watch.enable,env:WATCH_ENABLE" help:"Watch for package manager log file updates"`
	WatchInterval time.Duration `arg:"--watch.interval,env:WATCH_INTERVAL" help:"Time interval between package manager log file checks" default:"10s"`
}

func getLogLevels(level log.Level) []log.Level {
	ret := make([]log.Level, 0)
	for _, lvl := range log.AllLevels {
		if level >= lvl {
			ret = append(ret, lvl)
		}
	}
	return ret
}

func setupLogging() *os.File {
	var (
		logLevel log.Level
		err      error
		file     *os.File
	)
	if args.Systemd || args.Daemon {
		// Disable timestamps when running in background mode
		// They are not needed as these modes are most likely used with systemd
		// which adds its own timestamps
		log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})
	} else {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	}
	log.SetOutput(ioutil.Discard)
	if args.Debug {
		logLevel = log.DebugLevel
	} else {
		logLevel, err = log.ParseLevel(args.LogLevel)
		if err != nil {
			logLevel = log.InfoLevel
			log.Warnf("Unknown log level %s, defaulting to INFO", args.LogLevel)
		}
	}
	levels := getLogLevels(logLevel)
	if !args.Quiet {
		log.AddHook(&writer.Hook{
			Writer:    os.Stderr,
			LogLevels: levels,
		})
	}
	if !args.NoLogFile && args.LogFile != "" {
		file, err = os.OpenFile(args.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Error(err)
		} else {
			log.AddHook(&writer.Hook{
				Writer:    file,
				LogLevels: levels,
			})
		}
	}
	return file
}

func setupDistro() {
	// Setup distro specific stuff
	distro, err := getDistro()
	if err != nil {
		log.Fatalln(err)
	}
	switch distro {
	case "fedora":
		cache.updateFunc = UpdateDnf
		cache.logFp = "/var/log/dnf.rpm.log"
		cache.logFunc = checkDnfLogs
	case "arch":
		cache.logFp = "/var/log/pacman.log"
		cache.logFunc = checkPacmanLogs
		cache.updateFunc = UpdateArch
		for _, h := range supportedHelpers {
			if !checkCmd(h.name) {
				continue
			}
			if args.AurHelper != "" && h.name != args.AurHelper {
				log.Infof("%s is available but %s was requested", h.name, args.AurHelper)
				continue
			}
			aur = h
			break
		}
		if aur.name == "" {
			log.Warn("no supported AUR helper found")
		} else {
			log.Infof("AUR helper: %s", aur.name)
		}
	default:
		log.Fatalf("unsupported distro %s", distro)
	}
}

func setupCache() {
	if args.NoCache {
		cache.fp = ""
		log.Info("cache file disabled")
	} else if args.CacheFile != "" {
		cache.fp = args.CacheFile
		if checkFileRead(cache.fp) && !checkFileWrite(cache.fp) {
			log.Fatal("cache file is not writable")
		}
		log.Infof("cache file: %s", cache.fp)
	} else {
		log.Warnf("no cache file path set")
	}
}

func runDaemon(listener net.Listener) {
	if !args.NoRefresh {
		log.Infof("auto-refresh every %v", args.CacheInterval)
		go func() {
			ticker := time.NewTicker(args.CacheInterval)
			for {
				select {
				case <-ticker.C:
					log.Debug("auto-refresh ticker")
					if err := cache.Update(); err != nil {
						log.Error(err)
					}
				}
			}
		}()
	} else {
		log.Info("auto-refresh disabled")
	}
	if args.Watch {
		if cache.logFp == "" {
			log.Errorf("cannot watch, unsupported package manager")
		} else {
			log.Infof("watching %s, checking every %s", cache.logFp, args.WatchInterval)
			go cache.WatchLogs(args.WatchInterval)
		}
	}
	http.HandleFunc("/api", HandleAPI)
	http.HandleFunc("/ws", HandleWS)
	if cache.NeedsUpdate(args.CacheInterval) {
		if err := cache.Update(); err != nil {
			log.Errorf("refresh failed: %v", err)
		}
		log.Infof("found %d updates", len(cache.f.Updates))
	}
	log.Infof("listening on %s", listener.Addr().String())
	err := http.Serve(listener, nil)
	if err != http.ErrServerClosed {
		log.Errorf("HTTP serve error: %v", err)
		os.Exit(2)
	}
	os.Exit(0)
}

func runForeground() {
	if !cache.NeedsUpdate(args.CacheInterval) {
		log.Info("no update required")
		return
	}
	if err := cache.Update(); err != nil {
		log.Errorf("refresh failed: %v", err)
		return
	}
	// Print to console
	fmt.Print(cache.f.String())
}

func main() {
	args.CacheFile, _ = getCachePath()
	args.CacheInterval, _ = time.ParseDuration(defaultWait)
	arg.MustParse(&args)

	file := setupLogging()
	if file != nil {
		defer file.Close()
	}
	setupDistro()
	setupCache()

	if args.Systemd {
		listeners, err := activation.Listeners()
		if err != nil {
			log.Fatal(err)
		}
		if len(listeners) != 1 {
			log.Fatal("unexpected number of socket activation fds")
		}
		runDaemon(listeners[0])
	} else if args.Daemon {
		listener, err := net.Listen("tcp", args.ListenAddress)
		if err != nil {
			log.Fatalf("cannot listen: %s", err)
		}
		runDaemon(listener)
	} else {
		runForeground()
	}
}
