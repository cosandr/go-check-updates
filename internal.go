package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/cosandr/go-check-updates/api"
)

//  NewInternalCache returns a pointer to a new InternalCache struct
func NewInternalCache() *InternalCache {
	return &InternalCache{
		f:  api.File{},
		ws: &WsFeed{listeners: make(map[uint16]chan struct{})},
	}
}

// Subscription holds data for a listener
type Subscription struct {
	feed *WsFeed
	idx  uint16
	ch   chan struct{}
	once sync.Once
}

// Unsubscribe removes listener from feed
func (s *Subscription) Unsubscribe() {
	s.once.Do(func() {
		s.feed.remove(s.idx)
	})
}

// WsFeed holds data for waking up websocket goroutines
//
// Thanks to https://rauljordan.com/2019/09/23/how-to-write-an-event-feed-library.html
type WsFeed struct {
	L         sync.Mutex
	listeners map[uint16]chan struct{}
	count     uint16
}

func (f *WsFeed) remove(i uint16) {
	f.L.Lock()
	defer f.L.Unlock()
	delete(f.listeners, i)
	log.Debugf("WsFeed.remove: %d", i)
}

// Broadcast wakes up all listeners
func (f *WsFeed) Broadcast() {
	f.L.Lock()
	defer f.L.Unlock()
	var empty struct{}
	for idx, lis := range f.listeners {
		log.Debugf("WsFeed.Broadcast: %d", idx)
		lis <- empty
	}
}

// Subscribe registers new listener and returns its subscription
func (f *WsFeed) Subscribe() *Subscription {
	f.L.Lock()
	defer f.L.Unlock()
	ch := make(chan struct{}, 1)
	f.count++
	f.listeners[f.count] = ch
	log.Debugf("WsFeed.Subscribe: %d", f.count)
	return &Subscription{
		feed: f,
		idx:  f.count,
		ch:   ch,
	}
}

// InternalCache stores information about the updates cache
// Contains a WsFeed for threadsafe operations
type InternalCache struct {
	f          api.File
	fp         string
	logFp      string
	logFunc    func(string) error
	updateFunc func() (updates []api.Update, err error)
	ws         *WsFeed
}

// Update the internal cache and optional file
func (ic *InternalCache) Update() error {
	log.Info("refreshing")
	updates, err := ic.updateFunc()
	if err != nil {
		// Something failed and we got nothing
		if len(updates) == 0 {
			return err
		}
		// Partial failure, continue
		log.Error(err)
	}
	sort.Slice(updates, func(i, j int) bool {
		return updates[i].Pkg < updates[j].Pkg
	})
	ic.f.Updates = updates
	ic.f.Checked = time.Now().Format(time.RFC3339)
	if ic.fp != "" {
		err = ic.Write()
	}
	ic.ws.Broadcast()
	log.Debug("InternalCache.Update: WS broadcast")
	return err
}

// RefreshFromLogs updates cache by reading package manager logs
func (ic *InternalCache) RefreshFromLogs() error {
	if ic.logFp == "" {
		return fmt.Errorf("InternalCache.RefreshFromLogs: no package manager log file path")
	}
	if err := ic.logFunc(ic.logFp); err != nil {
		return err
	}
	ic.ws.Broadcast()
	log.Debug("InternalCache.RefreshFromLogs: WS broadcast")
	return nil
}

// WatchLogs checks the package manager log according to the interval
// and calls RefreshFromLogs if the file changed.
func (ic *InternalCache) WatchLogs(interval time.Duration) {
	var last time.Time
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			info, err := os.Stat(ic.logFp)
			if err != nil {
				log.Errorf("InternalCache.WatchLogs: %v", err)
				continue
			}
			if info.ModTime().Equal(last) {
				log.Debugf("InternalCache.WatchLogs: %s modified time unchanged", ic.logFp)
				continue
			}
			last = info.ModTime()
			err = ic.RefreshFromLogs()
			if err != nil {
				log.Error(err)
			}
		}
	}
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
