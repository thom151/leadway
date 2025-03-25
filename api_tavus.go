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

	//	"strings"

	"github.com/thom151/leadme/internal/database"
)

type VideoResponse struct {
	VideoID   string `json:"video_id"`
	VideoName string `json:"video_name"`
	Status    string `json:"status"`
	Data      struct {
		Script string `json:"script"`
	} `json:"data"`
	ReplicaID          string `json:"replica_id"`
	DownloadURL        string `json:"download_url"`
	HostedURL          string `json:"hosted_url"`
	StreamURL          string `json:"stream_url"`
	StatusDetails      string `json:"status_details"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	GenerationProgress string `json:"generation_progress"`
}

func (cfg *apiConfig) generateAvatar(user database.User, videoUrl string) (string, error) {
	url := "https://tavusapi.com/v2/replicas"

	payloadData := map[string]string{
		"replica_name":    fmt.Sprintf("%s-%s", user.Username, user.ID),
		"train_video_url": videoUrl,
	}

	payloadBytes, err := json.Marshal(payloadData)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("x-api-key", cfg.tavusApiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("returned: %s", res.StatusCode)
	}

	var responseData struct {
		ReplicaID string `json:"replica_id"`
		Status    string `json:"status"`
	}

	err = json.Unmarshal(body, &responseData)
	if err != nil {
		return "", err
	}

	log.Println("unmarshal successful\n")
	fmt.Println("Avatar Id: ", responseData.ReplicaID)

	return responseData.ReplicaID, nil
}

func (cfg *apiConfig) waitForTrainingComplete(replicaId string) error {
	client := &http.Client{}
	for {
		pollURL := fmt.Sprintf("https://tavusapi.com/v2/replicas/%s", replicaId)
		req, err := http.NewRequest("GET", pollURL, nil)
		if err != nil {
			return fmt.Errorf("create poll request: %v", err)
		}
		req.Header.Set("x-api-key", cfg.tavusApiKey)
		req.Header.Set("Content-Type", "application/json")

		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("poll request: %v", err)
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("read poll response: %v", err)
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("poll returned %d: %s", res.StatusCode, string(body))
		}

		// Log the raw body for debugging

		var responseData struct {
			ReplicaID string `json:"replica_id"`
			Status    string `json:"status"`
			Error     string `json:"error"`
		}
		if err := json.Unmarshal(body, &responseData); err != nil {
			return fmt.Errorf("unmarshal poll response: %v", err)
		}
		if responseData.Status == "error" {
			return fmt.Errorf("replica creation failed: %s", string(body))
		}
		if responseData.Status == "completed" {
			break
		}

		log.Printf("status: %s \n replica_id: %s\n\n", responseData.Status, responseData.ReplicaID)
		time.Sleep(5 * time.Second)
	}
	return nil
}

func (cfg *apiConfig) generateVideoAndGetId(aiResponse, replicaID string) (string, error) {
	url := "https://tavusapi.com/v2/videos"

	payloadData := map[string]string{
		"replica_id": replicaID,
		"script":     aiResponse,
	}

	payloadBytes, err := json.Marshal(payloadData)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", cfg.tavusApiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", res.StatusCode, string(body))
	}

	var responseData struct {
		VideoId string `json:"video_id"`
		Status  string `json:"status"`
	}

	err = json.Unmarshal(body, &responseData)
	if err != nil {
		return "", err
	}

	log.Println("Video id: ", responseData.VideoId)

	return responseData.VideoId, nil

}

func (cfg *apiConfig) downloadVideo(videoId, userId, taskId string) (string, error) {
	url := "https://tavusapi.com/v2/videos/" + videoId
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("x-api-key", cfg.tavusApiKey)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", res.StatusCode, string(body))
	}

	var videoResp VideoResponse
	for {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %v", err)
		}
		req.Header.Set("x-api-key", cfg.tavusApiKey)

		res, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("send request: %v", err)
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return "", fmt.Errorf("read response: %v", err)
		}
		if res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("API returned %d: %s", res.StatusCode, string(body))
		}

		log.Printf("API response: %s", string(body))
		if err := json.Unmarshal(body, &videoResp); err != nil {
			return "", fmt.Errorf("unmarshal response: %v", err)
		}

		if videoResp.Status == "ready" {
			break
		}
		if videoResp.Status == "error" {
			return "", fmt.Errorf("video generation failed")
		}
		log.Println("Video status:", videoResp.Status)
		time.Sleep(5 * time.Second) // Poll every 5 seconds
	}
	downloadUrl := videoResp.DownloadURL

	log.Println("Download url: ", downloadUrl)
	resp, err := http.Get(downloadUrl)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	taskDir := filepath.Join("temp", userId, taskId)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create task directory %s: %v", taskDir, err)
	}
	videoPath := filepath.Join(taskDir, "video.mp4")
	out, err := os.Create(videoPath)
	if err != nil {
		return "", err
	}

	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return videoPath, nil

}
