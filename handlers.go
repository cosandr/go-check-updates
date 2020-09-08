package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/cosandr/go-check-updates/api"
)

// HandleAPI returns updates or cache file location, one of filepath or updates params is required
//
// Mandatory params (at least one):
// - filepath: return file path
// - updates: return list of updates
// - refresh: refresh updates
// Optional params:
// - every: used with refresh, time duration to wait between updates
// - immediate: used with refresh, return response without waiting for update to finish
func HandleAPI(w http.ResponseWriter, r *http.Request) {
	log.Debugf("GET - %s - %s", r.RemoteAddr, r.RequestURI)
	w.Header().Set("Content-Type", contentType)
	var resp api.Response
	defer func() {
		log.Debug("Marshalling response")
		d, _ := json.Marshal(&resp)
		log.Debugf("Sending response:\n%s", string(d))
		_, _ = w.Write(d)
	}()
	params := r.URL.Query()
	_, updates := params["updates"]
	_, filepath := params["filepath"]
	_, refresh := params["refresh"]
	if !(updates || filepath || refresh) {
		log.Debug("Missing arguments")
		resp.Error = "filepath, updates and/or refresh parameter(s) required"
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if refresh {
		var willRefresh bool
		_, immediate := params["immediate"]
		// Sets willRefresh to true if an update is needed
		if val := params.Get("every"); val != "" {
			every, err := time.ParseDuration(val)
			if err != nil {
				resp.Error = fmt.Sprintf("Cannot parse time duration: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			log.Debug("Conditional cache file update requested")
			willRefresh = needsUpdate(every)
		} else {
			log.Debug("Unconditional cache file update requested")
			willRefresh = true
		}
		if willRefresh {
			log.Debug("Cache file refreshing")
			if immediate {
				globalWg.Add(1)
				go func() {
					_ = updateCache()
					globalWg.Done()
				}()
				log.Debug("Cache file update queued")
				tmp := true
				resp.Queued = &tmp
				w.WriteHeader(http.StatusAccepted)
			} else {
				log.Debug("Cache file updating")
				err := updateCache()
				if err != nil {
					log.Errorf("Update failed: %v", err)
					resp.Error = fmt.Sprintf("Cannot update cache file: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				log.Debug("Cache file updated")
			}
		}
	}
	if filepath {
		log.Debugf("File path requested: %s", cacheFilePath)
		resp.FilePath = cacheFilePath
	}
	if updates {
		log.Debug("Updates requested")
		if latestFile.f.IsEmpty() {
			log.Debugf("Latest file is empty, reading from %s", cacheFilePath)
			f, err := readCacheFile()
			if err != nil {
				log.Errorf("Read failed: %v", err)
				resp.Error += fmt.Sprintf("Cannot read cache file: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			log.Debug("File read, copying to latestFile")
			updateLatestFile(&f)
		}
		log.Debug("Setting response data to cache file content")
		resp.Data = &latestFile.f
	}
	return
}

// HandleWS sends notifications when updates are refreshed
func HandleWS(w http.ResponseWriter, r *http.Request) {
	log.Debugf("GET - %s - %s", r.RemoteAddr, r.RequestURI)
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go wsWriter(ctx, cancel, ws, &wg)
	wg.Add(1)
	go wsReader(ctx, cancel, ws, &wg)
	wg.Wait()
	ws.Close()
}
