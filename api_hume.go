package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Voice struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

type Utterance struct {
	Text        string  `json:"text"`
	Description string  `json:"description"`
	Speed       float64 `json:"speed"`
	Voice       Voice   `json:"voice"`
}

type Context struct {
	GenerationID string `json:"generation_id"`
}

type Format struct {
	Type string `json:"type"`
}

type HumePayload struct {
	Utterances     []Utterance `json:"utterances"`
	Context        Context     `json:"context"`
	Format         Format      `json:"format"`
	NumGenerations int         `json:"num_generations"`
}

func (cfg *apiConfig) generateAudioHume(generationId, voiceId, cloneResponse, description string) (bodyBytes []byte, err error) {
	url := "https://api.hume.ai/v0/tts/file"

	payloadData := HumePayload{
		Utterances: []Utterance{
			{
				Text:        cloneResponse,
				Description: "A warm and confident australian real estate agent that is talking to his clients with a natural rise and fall.", Speed: 1.1,
				Voice: Voice{
					Name:     voiceId,
					Provider: "CUSTOM_VOICE",
				},
			},
		},
		Context: Context{
			GenerationID: generationId,
		},
		Format: Format{
			Type: "mp3",
		},
		NumGenerations: 1,
	}

	payloadBytes, err := json.Marshal(payloadData)
	if err != nil {
		log.Println("error marshalling payload: ", err)
		return nil, err
	}

	payload := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		log.Print("err: %v", err.Error())
		return nil, err
	}
	req.Header.Add("X-Hume-Api-Key", cfg.humeApiKey)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Print("err: %v", err.Error())
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Print("err: %v", body)
		return nil, fmt.Errorf("Api error: %s", body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print("err: %v", err.Error())
		return nil, err
	}

	return body, nil
}
