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
	packageName   string = "go-check-updates"
	defaultWait   string = "12h"
	defaultNotify string = "1h"
)

var aur = helper{}
var cache = NewInternalCache()
var args struct {
	AurHelper      string        `arg:"--aur" help:"Override AUR helper (Arch Linux)"`
	CacheFile      string        `arg:"--cache.file,env:CACHE_FILE" help:"Path to update cache file"`
	CacheInterval  time.Duration `arg:"--cache.interval,env:CACHE_INTERVAL" help:"Time interval between cache updates"`
	Daemon         bool          `arg:"-d,--daemon" help:"Run as a daemon"`
	Debug          bool          `arg:"--debug,env:DEBUG" help:"Set console log output to DEBUG"`
	ListenAddress  string        `arg:"--web.listen-address,env:LISTEN_ADDRESS" help:"Web server listen address" default:":8100"`
	LogFile        string        `arg:"--log.file,env:LOG_FILE" help:"Path to log file"`
	LogLevel       string        `arg:"--log.level,env:LOG_LEVEL" default:"INFO" help:"Set log level"`
	NoCache        bool          `arg:"--no-cache,env:NO_CACHE" help:"Don't use cache file"`
	NoLogFile      bool          `arg:"--no-log,env:NO_LOG_FILE" help:"Don't log to file"`
	NoRefresh      bool          `arg:"--no-refresh,env:NO_REFRESH" help:"Don't auto-refresh"`
	Notify         bool          `arg:"--notify.enable,env:NOTIFY_ENABLE" help:"Enable notifications, webhook URL is required"`
	NotifyInterval time.Duration `arg:"--notify.interval,env:NOTIFY_INTERVAL" help:"Minimum time between notifications"`
	NotifyFormat   string        `arg:"--notify.format,env:NOTIFY_FORMAT" help:"Time format for embed footer" default:"2006/01/02 15:04"`
	Quiet          bool          `arg:"-q,--quiet" help:"Don't log to console"`
	Systemd        bool          `arg:"--systemd" help:"Run HTTP server using systemd socket activation"`
	Watch          bool          `arg:"-w,--watch.enable,env:WATCH_ENABLE" help:"Watch for package manager log file updates"`
	WatchInterval  time.Duration `arg:"--watch.interval,env:WATCH_INTERVAL" help:"Time interval between package manager log file checks" default:"10s"`
	WebhookURL     string        `arg:"--webhook-url,env:WEBHOOK_URL" help:"Discord Webhook URL"`
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
	case "fedora", "centos", "rhel":
		cache.updateFunc = UpdateDnf
		cache.logFp = "/var/log/dnf.rpm.log"
		cache.logFunc = checkDnfLogs
	case "arch", "manjaro":
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
		return
	} else if args.CacheFile == "" {
		log.Warn("no cache file path set")
		return
	}
	cache.fp = args.CacheFile
	if checkFileRead(cache.fp) && !checkFileWrite(cache.fp) {
		log.Fatal("cache file is not writable")
	}
	log.Infof("cache file: %s", cache.fp)
}

func setupAutoRefresh() {
	if args.NoRefresh {
		log.Info("auto-refresh disabled")
		return
	}
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
}

func setupWatch() {
	if !args.Watch {
		log.Info("watch disabled")
		return
	}
	if cache.logFp == "" {
		log.Error("cannot watch, unsupported package manager")
	} else {
		log.Infof("watching %s, checking every %s", cache.logFp, args.WatchInterval)
		go cache.WatchLogs(args.WatchInterval)
	}
}

func setupNotify() {
	if !args.Notify {
		log.Info("notifications disabled")
		return
	} else if args.WebhookURL == "" {
		log.Error("notifications disabled, missing Discord webhook URL")
		return
	}
	log.Infof("notify interval %v", args.NotifyInterval)
	go func() {
		sub := cache.ws.Subscribe()
		defer sub.Unsubscribe()
		prevUpdates := 0
		var prevNotify time.Time
		for {
			select {
			case <-sub.ch:
				log.Debug("notify received broadcast")
				curUpdates := len(cache.f.Updates)
				if curUpdates == prevUpdates || time.Since(prevNotify) < args.NotifyInterval {
					continue
				}
				log.Debugf("[notify] update count changed from %d to %d", prevUpdates, curUpdates)
				if err := sendUpdatesNotification(); err != nil {
					log.Warnf("failed to send notification: %v", err)
				}
				prevUpdates = curUpdates
				prevNotify = time.Now()
			}
		}
	}()
}

func runDaemon(listener net.Listener) {
	setupAutoRefresh()
	setupWatch()
	setupNotify()
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
	args.NotifyInterval, _ = time.ParseDuration(defaultNotify)
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
