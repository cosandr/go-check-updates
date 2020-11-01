package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"github.com/cosandr/go-check-updates/api"
)

var upgrader = websocket.Upgrader{}

// HandleAPI returns updates or cache file location, one of filepath or updates params is required
//
// Mandatory params (at least one):
// - filepath: return file path
// - updates: return list of updates
// - refresh: refresh updates
// Optional params:
// - log_file: used with refresh, read package manager log
// - every: used with refresh, time duration to wait between updates
// - immediate: used with refresh, return response without waiting for update to finish
func HandleAPI(w http.ResponseWriter, r *http.Request) {
	var start time.Time
	log.Debugf("HandleAPI: GET - %s - %s", r.RemoteAddr, r.RequestURI)
	if log.GetLevel() == log.DebugLevel {
		start = time.Now()
	}
	w.Header().Set("Content-Type", "application/json")
	var resp api.Response
	defer func() {
		if log.GetLevel() == log.DebugLevel {
			log.Debugf("HandleAPI: request done in %dms", time.Since(start).Milliseconds())
		}
		d, _ := json.Marshal(&resp)
		log.Debugf("HandleAPI: sending response:\n%s", string(d))
		_, _ = w.Write(d)
	}()
	params := r.URL.Query()
	_, updates := params["updates"]
	_, filepath := params["filepath"]
	_, refresh := params["refresh"]
	if !(updates || filepath || refresh) {
		log.Debug("HandleAPI: missing arguments")
		resp.Error = "filepath, updates and/or refresh parameter(s) required"
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if refresh {
		if _, useLog := params["log_file"]; useLog {
			log.Debug("HandleAPI: update from package manager log file")
			if cache.f.Checked == "" {
				resp.Error = fmt.Sprintf("Updates were never checked, cannot update from logs.")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := cache.RefreshFromLogs(); err != nil {
				log.Errorf("HandleAPI: %v", err)
				resp.Error = fmt.Sprintf("Cannot update from package manager logs: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
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
				log.Debug("HandleAPI: conditional cache file update requested")
				willRefresh = cache.NeedsUpdate(every)
			} else {
				log.Debug("HandleAPI: unconditional cache file update requested")
				willRefresh = true
			}
			if willRefresh {
				log.Debug("HandleAPI: cache file refreshing")
				if immediate {
					go cache.Update()
					log.Debug("HandleAPI: cache file update queued")
					tmp := true
					resp.Queued = &tmp
					w.WriteHeader(http.StatusAccepted)
				} else {
					log.Debug("HandleAPI: cache file updating")
					err := cache.Update()
					if err != nil {
						log.Errorf("HandleAPI: update failed: %v", err)
						resp.Error = fmt.Sprintf("Cannot update cache file: %v", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					log.Debug("HandleAPI: cache file updated")
				}
			}
		}
	}
	if filepath {
		log.Debugf("HandleAPI: file path requested: %s", cache.fp)
		resp.FilePath = cache.fp
	}
	if updates {
		log.Debug("HandleAPI: updates requested")
		f, err := cache.GetFile()
		if err != nil {
			log.Errorf("HandleAPI: %v", err)
			resp.Error += err.Error()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Debug("HandleAPI: setting response data to cache file content")
		resp.Data = &f
	}
	return
}

// HandleWS sends notifications when updates are refreshed
func HandleWS(w http.ResponseWriter, r *http.Request) {
	log.Debugf("HandleWS: GET - %s - %s", r.RemoteAddr, r.RequestURI)
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("HandleWS: upgrade: %v", err)
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
