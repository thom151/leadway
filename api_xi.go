package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// OKAY FAST ENOUGH
func generateAudio(cfg *apiConfig, voiceID, cloneResponse string) ([]byte, error) {
	url := "https://api.elevenlabs.io/v1/text-to-speech/" + voiceID + "/stream?output_format=mp3_44100_64"

	// Properly escape the JSON payload
	payloadData := map[string]string{
		"text":     cloneResponse,
		"model_id": "eleven_multilingual_v2",
	}
	payloadBytes, err := json.Marshal(payloadData)
	if err != nil {
		log.Println("error marshalling payload:", err)
		return nil, fmt.Errorf("error marshalling payload: %w", err)
	}
	payload := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Add("xi-api-key", cfg.elevenApiKey)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "LeadMeApp/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "audio/") {
		log.Println("Unexpected Content-Type:", contentType)
		log.Println("Response body:", string(body))
		return nil, fmt.Errorf("expected audio response, got Content-Type: %s", contentType)
	}

	return body, nil
}

func createElevenConnection(cfg *apiConfig, voiceID string) (*websocket.Conn, error) {
	ctx := context.Background()

	url := "wss://api.elevenlabs.io/v1/text-to-speech/" + voiceID + "/stream-input?output_format=ulaw_8000&auto_mode=true"

	headers := http.Header{}
	headers.Set("xi-api-key", cfg.elevenApiKey)
	xiConn, _, err := websocket.DefaultDialer.DialContext(ctx, url, headers)
	if err != nil {
		log.Println("error connecting to elevenlabs", err)
		return nil, err
	}
	initialMsg := map[string]interface{}{
		"text": " ", // Required to start the stream
		"voice_settings": map[string]interface{}{
			"stability":        0.5,
			"similarity_boost": 0.8,
			"speed":            1.0,
		},
	}
	err = xiConn.WriteJSON(initialMsg)
	if err != nil {
		log.Println("error sending initial message to ElevenLabs:", err)
		xiConn.Close()
		return nil, err
	}

	return xiConn, nil
}

// OK FAST ENOUGH
func sendAudioToTwilio(conn *websocket.Conn, audio []byte, streamSid string) error {
	base64Audio := base64.StdEncoding.EncodeToString(audio)

	twilioMsg := map[string]interface{}{
		"event":     "media",
		"streamSid": streamSid,
		"media": map[string]interface{}{
			"payload": base64Audio,
		},
	}

	err := conn.WriteJSON(twilioMsg)
	if err != nil {
		log.Println("error writing to connection/websocket", err)
		return err
	}

	fmt.Println("successfull \n\n")
	return nil
}
