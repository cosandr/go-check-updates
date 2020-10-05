package main

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	// Time allowed to write the file to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

func wsReader(ctx context.Context, cancel context.CancelFunc, ws *websocket.Conn, wg *sync.WaitGroup) {
	remoteName := ws.RemoteAddr().String()
	defer func() {
		log.Debugf("wsReader (%s): close", remoteName)
		cancel()
		wg.Done()
	}()
	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		select {
		case <-ctx.Done():
			log.Debugf("wsReader (%s): closed externally", remoteName)
			return
		default:
			_, _, err := ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
					log.Warnf("wsReader (%s): could not read Pong: %v", remoteName, err)
				}
				cancel()
				return
			}
		}
	}
}

func wsWriter(ctx context.Context, cancel context.CancelFunc, ws *websocket.Conn, wg *sync.WaitGroup) {
	remoteName := ws.RemoteAddr().String()
	sub := cache.ws.Subscribe()
	defer func() {
		sub.Unsubscribe()
		log.Debugf("wsWriter (%s): close", remoteName)
		cancel()
		wg.Done()
	}()
	// Ping-Pong goroutine
	go func() {
		pingTicker := time.NewTicker(pingPeriod)
		defer pingTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Debugf("wsWriter (%s): heartbeat closed externally", remoteName)
				return
			case <-pingTicker.C:
				log.Debugf("wsWriter (%s): sending heartbeat", remoteName)
				ws.SetWriteDeadline(time.Now().Add(writeWait))
				err := ws.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
						log.Errorf("wsWriter (%s): cannot send heartbeat: %v", remoteName, err)
					}
					cancel()
					return
				}
			}
		}
	}()
	// Main loop, wait for event and send updates to client
	for {
		select {
		case <-ctx.Done():
			log.Debugf("wsWriter (%s): message sender closed externally", remoteName)
			return
		case <-sub.ch:
			log.Debugf("wsWriter (%s): sending message", remoteName)
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			err := ws.WriteJSON(&cache.f)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
					log.Errorf("wsWriter (%s): cannot send message: %v", remoteName, err)
				}
				cancel()
				return
			}
		}
	}
}
