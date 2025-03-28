package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

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
