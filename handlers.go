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
	w.Header().Set("Content-Type", "application/json")
	var resp api.Response
	defer func() {
		log.Debug("HandleAPI: Marshalling response")
		d, _ := json.Marshal(&resp)
		log.Debugf("HandleAPI: Sending response:\n%s", string(d))
		_, _ = w.Write(d)
	}()
	params := r.URL.Query()
	_, updates := params["updates"]
	_, filepath := params["filepath"]
	_, refresh := params["refresh"]
	if !(updates || filepath || refresh) {
		log.Debug("HandleAPI: Missing arguments")
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
			log.Debug("HandleAPI: Conditional cache file update requested")
			willRefresh = cache.NeedsUpdate(every)
		} else {
			log.Debug("HandleAPI: Unconditional cache file update requested")
			willRefresh = true
		}
		if willRefresh {
			log.Debug("HandleAPI: Cache file refreshing")
			if immediate {
				globalWg.Add(1)
				go func() {
					_ = cache.Update()
					globalWg.Done()
				}()
				log.Debug("HandleAPI: Cache file update queued")
				tmp := true
				resp.Queued = &tmp
				w.WriteHeader(http.StatusAccepted)
			} else {
				log.Debug("HandleAPI: Cache file updating")
				err := cache.Update()
				if err != nil {
					log.Errorf("Update failed: %v", err)
					resp.Error = fmt.Sprintf("Cannot update cache file: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				log.Debug("HandleAPI: Cache file updated")
			}
		}
	}
	if filepath {
		log.Debugf("HandleAPI: File path requested: %s", cache.fp)
		resp.FilePath = cache.fp
	}
	if updates {
		log.Debug("HandleAPI: Updates requested")
		f, err := cache.GetFile()
		if err != nil {
			log.Errorf("HandleAPI: %v", err)
			resp.Error += err.Error()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Debug("HandleAPI: Setting response data to cache file content")
		resp.Data = &f
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
