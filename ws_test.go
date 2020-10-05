package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/cosandr/go-check-updates/api"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

func TestWS(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cache = InternalCache{
		Cond: sync.NewCond(&sync.Mutex{}),
		f:    api.File{},
	}
	addr := "127.0.0.1:1234"
	clientAddr := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
	interrupt := make(chan struct{})
	numClients := 5
	numIter := 2
	var wg sync.WaitGroup
	var httpWg sync.WaitGroup
	// Number of updates sent
	sent := make([]int, numIter)
	recv := make([][]int, numClients)

	httpWg.Add(1)
	srv := runServer(addr, &httpWg)
	// Wait for server to start
	time.Sleep(time.Second)

	for i := 0; i < numClients; i++ {
		recv[i] = make([]int, 0)
		go runClient(clientAddr.String(), interrupt, i, &wg, &recv[i])
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for i := 0; i < numIter; i++ {
		select {
		case <-ticker.C:
			// Generate some updates
			var updates []api.Update
			for j := 0; j < rand.Intn(100); j++ {
				updates = append(updates, api.Update{
					Pkg:    fmt.Sprintf("Iter %d package %d", i, j),
					NewVer: fmt.Sprintf("Iter %d new version %d", i, j),
				})
			}
			cache.f.Updates = updates
			cache.f.Checked = time.Now().Format(time.RFC3339)
			cache.Broadcast()
			sent[i] = len(updates)
			log.Infof("Set new updates iter %d", i)
			newDur := time.Duration(rand.Intn(10)) * time.Second
			log.Infof("Ticker duration changed to %.0f seconds", newDur.Seconds())
			ticker = time.NewTicker(newDur)
		}
	}
	// Close server
	srv.Shutdown(context.TODO())
	httpWg.Wait()

	close(interrupt)
	wg.Wait()
	// Check data
	log.Debugf("Sent: %v", sent)
	log.Debugf("Received: %v", recv)
	for i, r := range recv {
		if len(r) != len(sent) {
			t.Errorf("Client %d got updates %d times, expected %d", i, len(r), len(sent))
			continue
		}
		for j, n := range r {
			if sent[j] != n {
				t.Errorf("Client %d got %d updates, expected %d", i, n, sent[j])
			}
		}
	}
}

func runServer(addr string, wg *sync.WaitGroup) *http.Server {
	srv := &http.Server{Addr: addr}
	http.HandleFunc("/ws", HandleWS)
	go func() {
		defer wg.Done()
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			log.Fatalf("serve error: %v", err)
		}
	}()
	log.Infof("Listening on %s", addr)
	return srv
}

func runClient(addr string, interrupt chan struct{}, num int, wg *sync.WaitGroup, recv *[]int) {
	wg.Add(1)
	defer wg.Done()
	c, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})
	ping := make(chan struct{})
	var empty struct{}
	log.Infof("Client %d connected to %s", num, addr)

	go func() {
		defer close(done)
		for {
			msgType, message, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
					log.Errorf("Client %d read: %v", num, err)
				}
				return
			}
			switch msgType {
			case websocket.PingMessage:
				log.Infof("Client %d received ping", num)
				ping <- empty
			default:
				var data api.File
				err = json.Unmarshal(message, &data)
				if err != nil {
					log.Warnf("Client %d cannot unmarshal message: %s", num, message)
				} else {
					log.Infof("Client %d got %d updates", num, len(data.Updates))
					*recv = append(*recv, len(data.Updates))
				}
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-ping:
			log.Infof("Client %d sending pong", num)
			err := c.WriteMessage(websocket.PongMessage, nil)
			if err != nil {
				log.Errorf("Client %d write: %v", num, err)
				return
			}
		case <-interrupt:
			log.Infof("Client %d interrupt", num)

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Errorf("Client %d write close: %v", num, err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
