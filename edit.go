package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func (cfg *apiConfig) editAndUpload(video, broll, userId string, ts []float64) (string, error) {
	outputFile := userId + "-edited.mp4"

	if len(ts) != 2 || ts[0] >= ts[1] {
		return "", fmt.Errorf("invalid timestamps: must have exactly two timestamps with ts[0] < ts[1]")
	}

	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", video)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get video duration: %v", err)
	}
	totalDuration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse duration: %v", err)
	}
	log.Println("Total Duration: \n", totalDuration)

	//	broll_duration := ts[1] - ts[0]
	cmd = exec.Command("ffmpeg", "-ss", "0", "-i", video, "-t", fmt.Sprintf("%.2f", ts[0]),
		"-c", "copy", outputFile)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to extract segment1: %v", err)
	}
	return outputFile, nil
}

func (cfg *apiConfig) getCutTimestamps(audio string, aiResp openaiSmartResponse) ([]float64, error) {
	url := "https://api.deepgram.com/v1/listen?smart_format=true"

	file, err := os.Open(audio)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	req, err := http.NewRequest("POST", url, file)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Token "+cfg.deepgramApiKey)
	req.Header.Set("Content-Type", "audio/mpeg")

	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var dgSmartResp deepgramSmartResponse
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&dgSmartResp)
	if err != nil {
		return nil, err
	}

	if len(dgSmartResp.Results.Channels) == 0 || len(dgSmartResp.Results.Channels[0].Alternatives) == 0 {
		return nil, fmt.Errorf("no transcript results from deepgram")
	}

	var timestamps []float64
	for index := range aiResp.CutWords {
		indexTime := dgSmartResp.Results.Channels[0].Alternatives[0].Words[index].Start
		timestamps = append(timestamps, indexTime)
	}

	if len(timestamps) != 2 {
		return nil, err
	}
	return timestamps, nil
}

type deepgramSmartResponse struct {
	Metadata struct {
		TransactionKey string    `json:"transaction_key"`
		RequestID      string    `json:"request_id"`
		Sha256         string    `json:"sha256"`
		Created        time.Time `json:"created"`
		Duration       float64   `json:"duration"`
		Channels       int       `json:"channels"`
		Models         []string  `json:"models"`
		ModelInfo      struct {
			NAMING_FAILED struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				Arch    string `json:"arch"`
			} `json:""`
		} `json:"model_info"`
	} `json:"metadata"`
	Results struct {
		Channels []struct {
			Alternatives []struct {
				Transcript string  `json:"transcript"`
				Confidence float64 `json:"confidence"`
				Words      []struct {
					Word           string  `json:"word"`
					Start          float64 `json:"start"`
					End            float64 `json:"end"`
					Confidence     float64 `json:"confidence"`
					PunctuatedWord string  `json:"punctuated_word"`
				} `json:"words"`
				Paragraphs struct {
					Transcript string `json:"transcript"`
					Paragraphs []struct {
						Sentences []struct {
							Text  string  `json:"text"`
							Start float64 `json:"start"`
							End   float64 `json:"end"`
						} `json:"sentences"`
						NumWords int     `json:"num_words"`
						Start    float64 `json:"start"`
						End      float64 `json:"end"`
					} `json:"paragraphs"`
				} `json:"paragraphs"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
}
