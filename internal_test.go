package main

import (
	"io/ioutil"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/cosandr/go-check-updates/api"
)

func TestWatchLogs(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cache = InternalCache{
		f: api.File{
			Checked: "2020-05-29T23:00:00+02:00",
		},
		logFp:   "/tmp/test_watch.log",
		logFunc: checkPacmanLogs,
		ws:      &WsFeed{listeners: make(map[uint16]chan struct{})},
	}
	write := func(content string) error {
		err := ioutil.WriteFile(cache.logFp, []byte(content), 0644)
		if err != nil {
			return err
		}
		return nil
	}
	allUpdates := []api.Update{
		{
			Pkg:    "shellcheck",
			NewVer: "0.7.1-33",
		},
		{
			Pkg:    "vte-common",
			NewVer: "0.60.3-1",
		},
		{
			Pkg:    "vte3",
			NewVer: "0.60.3-1",
		},
	}
	cache.f.Updates = allUpdates
	go cache.WatchLogs(time.Second)
	file := `
[2020-05-29T23:47:18+0200] [ALPM] upgraded shellcheck (0.7.1-32 -> 0.7.1-33)
[2020-05-29T23:47:18+0200] [ALPM] upgraded vte-common (0.60.2-2 -> 0.60.3-1)
[2020-05-29T23:47:18+0200] [ALPM] upgraded vte3 (0.60.2-2 -> 0.60.3-1)
`
	if err := write(file); err != nil {
		t.Error(err)
	}
	// Wait for ticker
	time.Sleep(2 * time.Second)
	if len(cache.f.Updates) > 0 {
		t.Errorf("Expected 0 updates, got %d: %v", len(cache.f.Updates), cache.f.Updates)
	}
	// Only upgrade 1 of them
	cache.f.Updates = allUpdates
	file = `
[2020-05-29T23:47:18+0200] [ALPM] upgraded shellcheck (0.7.1-32 -> 0.7.1-33)
`
	if err := write(file); err != nil {
		t.Error(err)
	}
	time.Sleep(2 * time.Second)
	if len(cache.f.Updates) != 2 {
		t.Errorf("Expected 2 updates, got %d: %v", len(cache.f.Updates), cache.f.Updates)
	}
}
