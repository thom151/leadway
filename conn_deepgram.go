package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

func createDeepgramConnection(apiKey string) (*websocket.Conn, error) {
	ctx := context.Background()
	deepgraumURL := "wss://api.deepgram.com/v1/listen?encoding=mulaw&sample_rate=8000&channels=1&interim_results=true&filler_words=true&smart_format=true&utterance_end=500"
	dgConn, _, err := websocket.DefaultDialer.DialContext(ctx, deepgraumURL, http.Header{
		"Authorization": []string{"Token " + apiKey},
	})

	if err != nil {
		log.Println("error connecting deepgram", err)
		return nil, err
	}

	return dgConn, nil
}
