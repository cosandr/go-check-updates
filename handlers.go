package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	log.Debug(r.RequestURI)
	w.Header().Set("Content-Type", contentType)
	var resp api.Response
	params := r.URL.Query()
	_, updates := params["updates"]
	_, filepath := params["filepath"]
	_, refresh := params["refresh"]
	validQuery := updates || filepath || refresh
	if !validQuery {
		resp.Error = "filepath, updates and/or refresh parameter(s) required"
		w.WriteHeader(http.StatusBadRequest)
	} else {
		if refresh {
			var willRefresh bool
			_, immediate := params["immediate"]
			// Sets willRefresh to true if an update is needed
			if val := params.Get("every"); val != "" {
				every, err := time.ParseDuration(val)
				if err != nil {
					resp.Error = fmt.Sprintf("Cannot parse time duration: %v", err)
					w.WriteHeader(http.StatusBadRequest)
				} else {
					log.Debugf("Cache file update requested")
					willRefresh = needsUpdate(every)
				}
			} else {
				willRefresh = true
			}
			if willRefresh {
				if immediate {
					wg.Add(1)
					go func() {
						_ = updateCache()
						wg.Done()
					}()
					log.Debug("Update queued")
					tmp := true
					resp.Queued = &tmp
					w.WriteHeader(http.StatusAccepted)
				} else if err := updateCache(); err != nil {
					resp.Error = fmt.Sprintf("Cannot update cache file: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}
		}
		if _, ok := params["filepath"]; ok {
			resp.FilePath = cacheFilePath
		}
		if _, ok := params["updates"]; ok {
			f, err := readCache()
			if err != nil {
				resp.Error += fmt.Sprintf("Cannot read cache file: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				resp.Data = &f
			}
		}
	}
	d, _ := json.Marshal(&resp)
	log.Debugf("Sending response:\n%s", string(d))
	_, _ = w.Write(d)
}
