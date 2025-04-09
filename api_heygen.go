package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/thom151/leadme/internal/database"
)

func (cfg *apiConfig) generateVideoHeygen(avatarId string, series database.VideoSeries) (string, error) {
	url := "https://api.heygen.com/v2/video/generate"
	if series.AudioS3 == "unset" {
		return "", fmt.Errorf("missing audio url")
	}
	payload := VideoRequest{
		Caption: false,
		Title:   series.Title,
		VideoInputs: []VideoInput{
			{
				Character: &CharacterSettings{
					Type:        "avatar",
					AvatarID:    avatarId,
					Scale:       1.0,
					AvatarStyle: "normal",
				},
				Voice: VoiceSettings{
					Type:     "audio",
					AudioURL: series.AudioS3,
				},
			},
		},
		Dimension: Dimension{
			Width:  1280,
			Height: 720,
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	log.Println("Getting request from heygen")

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", cfg.heygenApiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing request: %w", err)
	}
	defer res.Body.Close()

	log.Println("response got")
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		return string(body), fmt.Errorf("API returned non-200 status: %s", res.Status)
	}

	var videoRes VideoResponseHeyGen
	err = json.Unmarshal(body, &videoRes)
	if err != nil {
		return "", err
	}

	if videoRes.Error != nil {
		return "", fmt.Errorf("API error: %s - %s", videoRes.Error.Code, videoRes.Error.Message)
	}

	return videoRes.Data.VideoID, nil

}

func downloadHeygenVideo(shareURL, outputPath string) error {
	dir := filepath.Dir(outputPath)
	safeOutputPath, err := safePath(dir, outputPath)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("error creating directories: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(shareURL)
	if err != nil {
		return fmt.Errorf("error downloading video: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	out, err := os.OpenFile(safeOutputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", safeOutputPath, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

func (cfg *apiConfig) getVideoStatus(videoID string) (string, error) {
	url := fmt.Sprintf("https://api.heygen.com/v1/video_status.get?video_id=%s", videoID)
	client := &http.Client{Timeout: 30 * time.Second}

	for {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("creating status request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Api-Key", cfg.heygenApiKey)

		res, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("executing status request: %w", err)
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return "", fmt.Errorf("reading status response: %w", err)
		}

		if res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("status API returned non-200: %s - %s", res.Status, string(body))
		}

		var statusResp struct {
			Data struct {
				Status   string `json:"status"`
				VideoURL string `json:"video_url"`
			} `json:"data"`
		}
		err = json.Unmarshal(body, &statusResp)
		if err != nil {
			return "", fmt.Errorf("unmarshaling status response: %w", err)
		}

		switch statusResp.Data.Status {
		case "completed":
			return statusResp.Data.VideoURL, nil
		case "pending", "processing", "waiting":
			fmt.Printf("Video %s status: %s, checking again in 5 seconds...\n", videoID, statusResp.Data.Status)
			time.Sleep(5 * time.Second)
		case "failed":
			return "", fmt.Errorf("video generation failed")
		default:
			return "", fmt.Errorf("unknown status: %s", statusResp.Data.Status)
		}

		log.Println("Status: ", statusResp.Data.Status)
	}
}

type VideoRequest struct {
	Caption     bool         `json:"caption,omitempty"`
	Title       string       `json:"title,omitempty"`
	VideoInputs []VideoInput `json:"video_inputs"`
	Dimension   Dimension    `json:"dimension"`
}

type VideoInput struct {
	Character  *CharacterSettings  `json:"character,omitempty"`
	Voice      VoiceSettings       `json:"voice"`
	Background *BackgroundSettings `json:"background,omitempty"`
}

type CharacterSettings struct {
	Type        string  `json:"type"`
	AvatarID    string  `json:"avatar_id"`
	Scale       float64 `json:"scale,omitempty"`
	AvatarStyle string  `json:"avatar_style,omitempty"`
}

type VoiceSettings struct {
	Type     string `json:"type"`
	AudioURL string `json:"audio_url,omitempty"`
}

type BackgroundSettings struct {
	Type  string `json:"type"`
	Value string `json:"value,omitempty"`
}

type Dimension struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type VideoResponseHeyGen struct {
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Data struct {
		VideoID string `json:"video_id"`
	} `json:"data"`
}

type ShareRequest struct {
	VideoID string `json:"video_id"`
}

type ShareResponse struct {
	Code    int    `json:"code"`
	Data    string `json:"data"`
	Msg     any    `json:"msg"`
	Message any    `json:"message"`
}
