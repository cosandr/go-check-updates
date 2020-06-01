package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cosandr/go-check-updates/api"
)

// HandleRun checks for updates immediately
//
// POST will update cache file, return immediately if immediate param is present
// GET will return pending updates list
func HandleRun(w http.ResponseWriter, r *http.Request) {
	if debug {
		log.Println(r.RequestURI)
	}
	w.Header().Set("Content-Type", contentType)
	var resp api.Response
	params := r.URL.Query()
	switch r.Method {
	case "GET":
		log.Println("Response requested")
		updates, err := runFunc()
		if err != nil {
			resp.Error = fmt.Sprintf("WARNING: %s", err)
		}
		resp.Data = &api.File{
			Checked: time.Now().Format(time.RFC3339),
			Updates: updates,
		}
	case "POST":
		resp.FilePath = cacheFilePath
		if _, ok := params["immediate"]; ok {
			wg.Add(1)
			go func() {
				_ = updateFile()
				wg.Done()
			}()
			log.Println("Update queued")
			*resp.Queued = true
		} else {
			log.Println("Update requested")
			err := updateFile()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				resp.Error = fmt.Sprintf("Update failed: %v", err)
			}
			log.Println("Update completed")
		}
	default:
		w.WriteHeader(http.StatusBadRequest)
		log.Printf("Bad request %s\n", r.Method)
		resp.Error = "POST to cache to file, GET to return updates"
	}
	d, _ := json.Marshal(&resp)
	if debug {
		log.Printf("Sending response:\n%s", string(d))
	}
	_, _ = w.Write(d)
}

// HandleCached returns the latest cached updates
//
// Params
// every: update before returning if file is older than this
// immediate: return immediately
func HandleCached(w http.ResponseWriter, r *http.Request) {
	if debug {
		log.Println(r.RequestURI)
	}
	w.Header().Set("Content-Type", contentType)
	var resp api.Response
	params := r.URL.Query()
	// Do we need to check file age?
	if val := params.Get("every"); val != "" {
		// Try to parse given duration
		every, err := time.ParseDuration(val)
		if err != nil {
			resp.Error = fmt.Sprintf("Cannot parse time duration: %v", err)
		} else {
			log.Printf("Cache file update requested")
			if needsUpdate(cacheFilePath, every) {
				// Is the immediate key in params?
				if _, ok := params["immediate"]; ok {
					wg.Add(1)
					go func() {
						_ = updateFile()
						wg.Done()
					}()
					log.Println("Update queued")
					*resp.Queued = true
				} else if err := updateFile(); err != nil {
					resp.Error = fmt.Sprintf("Cannot update cache file: %v", err)
				}
			}
		}
	}
	cacheFile, err := getCacheFile(cacheFilePath)
	if err != nil {
		resp.Error += fmt.Sprintf("Cannot open cache file: %v", err)
	} else {
		defer cacheFile.Close()
		f, err := readFile(cacheFile)
		if err != nil {
			resp.Error = fmt.Sprintf("Cannot parse cache file: %v", err)
		} else {
			resp.Data = &f
		}
	}
	d, _ := json.Marshal(&resp)
	if debug {
		log.Printf("Sending response:\n%s", string(d))
	}
	_, _ = w.Write(d)
}
