package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"sync"
	"time"

	"github.com/cosandr/go-check-updates/api"
	log "github.com/sirupsen/logrus"
)

// InternalCache stores information about the updates cache
// Contains a sync.Cond for threadsafe operations
type InternalCache struct {
	*sync.Cond
	f  api.File
	fp string
}

// Update the internal cache and optional file
func (ic *InternalCache) Update() error {
	log.Info("Refreshing")
	var updates []api.Update
	var err error
	switch distro {
	case "fedora":
		updates, err = UpdateDnf()
	case "arch":
		updates, err = UpdateArch()
	default:
		return fmt.Errorf("unsupported distro %s", distro)
	}
	if err != nil {
		return err
	}
	sort.Slice(updates, func(i, j int) bool {
		return updates[i].Pkg < updates[j].Pkg
	})
	ic.f.Updates = updates
	ic.f.Checked = time.Now().Format(time.RFC3339)
	if ic.fp != "" {
		err = ic.Write()
	}
	ic.Broadcast()
	log.Debug("InternalCache.Update: broadcast")
	return nil
}

// GetFile returns a copy of internal cache file
// If there it is empty, attempts to read it from disk
func (ic *InternalCache) GetFile() (api.File, error) {
	// log.Debugf("InternalCache.GetFile: %v", ic.f)
	if ic.f.IsEmpty() {
		if ic.fp != "" {
			return api.File{}, fmt.Errorf("cache is empty and cache file is disabled")
		} else if checkFileRead(ic.fp) {
			log.Debugf("InternalCache.GetFile, cache empty, reading from %s", ic.fp)
			err := ic.Read()
			if err != nil {
				return api.File{}, fmt.Errorf("cache is empty and cache file cannot be read: %v", err)
			}
		} else {
			return api.File{}, fmt.Errorf("cache is empty and no cache file was found")
		}
	}
	return ic.f.Copy(), nil
}

// NeedsUpdate returns true if the cache needs updating according to the update interval
//
// Malformed files are considered invalid and will be replaced
func (ic *InternalCache) NeedsUpdate(interval time.Duration) bool {
	if ic.f.IsEmpty() {
		if ic.fp == "" {
			return true
		}
		err := ic.Read()
		// Cannot read, update
		if err != nil {
			return true
		}
	}
	log.Infof("InternalCache.NeedsUpdate: last update: %s", ic.f.Checked)
	t, err := time.Parse(time.RFC3339, ic.f.Checked)
	// Can't parse timestamp, update
	if err != nil {
		return true
	}
	if time.Since(t) > interval {
		return true
	}
	return false
}

// Read file to internal cache
func (ic *InternalCache) Read() error {
	if ic.fp == "" {
		return fmt.Errorf("cache file disabled")
	}
	bytes, err := ioutil.ReadFile(ic.fp)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, &ic.f)
}

// Write internal cache to file
func (ic *InternalCache) Write() error {
	if ic.fp == "" {
		return fmt.Errorf("cache file disabled")
	}
	log.Debug("InternalCache.Write: marshal file")
	bytes, err := json.Marshal(&ic.f)
	if err != nil {
		return err
	}
	log.Debugf("InternalCache.Write: write file %s", ic.fp)
	return ioutil.WriteFile(ic.fp, bytes, 0644)
}
