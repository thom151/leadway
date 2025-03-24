package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (cfg *apiConfig) editAndUpload(video, broll, userId string, ts []float64) (string, error) {
	outputFile := userId + "-edited.mp4"

	if len(ts) != 2 || ts[0] >= ts[1] {
		return "", fmt.Errorf("invalid timestamps: must have exactly two timestamps with ts[0] < ts[1]")
	}

	var out, stderr bytes.Buffer

	// Get total video duration
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", video)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get video duration: %v, stderr: %s", err, stderr.String())
	}
	totalDuration, err := strconv.ParseFloat(strings.TrimSpace(out.String()), 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse duration: %v", err)
	}
	log.Println("Total Duration:", totalDuration)

	// Paths
	segment1 := filepath.Join("temp", "segment1.mp4")
	segment2 := filepath.Join("temp", "segment2.mp4")
	segment3 := filepath.Join("temp", "segment3.mp4")
	audioFile := filepath.Join("temp", "full_audio.aac")
	brollAudio := filepath.Join("temp", "broll_audio.aac")

	// Segment 1: with fade out
	fadeOutStart := ts[0] - 1
	if fadeOutStart < 0 {
		fadeOutStart = 0
	}
	fadeOutFilter := fmt.Sprintf("fade=out:st=%.2f:d=1", fadeOutStart)
	cmd = exec.Command("ffmpeg", "-ss", "0", "-i", video, "-t", fmt.Sprintf("%.2f", ts[0]),
		"-vf", fadeOutFilter, "-c:v", "libx264", "-c:a", "aac", "-y", segment1)
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to extract segment1: %v, stderr: %s", err, stderr.String())
	}

	// Extract full original audio
	cmd = exec.Command("ffmpeg", "-y", "-i", video, "-vn", "-acodec", "copy", audioFile)
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to extract audio: %v, stderr: %s", err, stderr.String())
	}

	// Trim audio for b-roll (RE-ENCODED to avoid EOF issue)
	brollDuration := ts[1] - ts[0]
	cmd = exec.Command("ffmpeg", "-y", "-ss", fmt.Sprintf("%.2f", ts[0]), "-t", fmt.Sprintf("%.2f", brollDuration),
		"-i", audioFile, "-c:a", "aac", brollAudio)
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to trim b-roll audio: %v, stderr: %s", err, stderr.String())
	}

	// Ensure audio was created and isn't empty
	stat, err := os.Stat(brollAudio)
	if err != nil || stat.Size() == 0 {
		return "", fmt.Errorf("b-roll audio file is missing or empty after trimming")
	}

	// Segment 2: b-roll with fade-in and overlaid voice audio
	cmd = exec.Command("ffmpeg", "-y", "-i", broll, "-i", brollAudio,
		"-vf", "fade=in:st=0:d=1", "-c:v", "libx264", "-c:a", "aac",
		"-map", "0:v:0", "-map", "1:a:0", segment2)
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create segment2 with b-roll and audio: %v, stderr: %s", err, stderr.String())
	}

	// Segment 3: from ts[1] to end
	cmd = exec.Command("ffmpeg", "-ss", fmt.Sprintf("%.2f", ts[1]), "-i", video,
		"-c:v", "libx264", "-c:a", "aac", "-y", segment3)
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to extract segment3: %v, stderr: %s", err, stderr.String())
	}

	// Create concat list
	concatList := filepath.Join("", "concat.txt")
	f, err := os.Create(concatList)
	if err != nil {
		return "", fmt.Errorf("failed to create concat list: %v", err)
	}
	defer f.Close()

	abs1, _ := filepath.Abs(segment1)
	abs2, _ := filepath.Abs(segment2)
	abs3, _ := filepath.Abs(segment3)
	_, err = f.WriteString(fmt.Sprintf("file '%s'\nfile '%s'\nfile '%s'\n", abs1, abs2, abs3))
	if err != nil {
		return "", fmt.Errorf("failed to write concat list: %v", err)
	}

	// Final concatenation
	cmd = exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", concatList, "-c", "copy", outputFile)
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to concatenate final video: %v, stderr: %s", err, stderr.String())
	}

	log.Println("Final video created at:", outputFile)
	return outputFile, nil
}

func (cfg *apiConfig) getCutTimestamps(audio string, aiResp openaiSmartResponse) ([]float64, error) {
	log.Println("Ai Smart Response: ", aiResp.CutWords)
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
	for _, word := range aiResp.CutWords {
		indexTime := dgSmartResp.Results.Channels[0].Alternatives[0].Words[word.Index].Start
		timestamps = append(timestamps, indexTime)
	}

	if len(timestamps) != 2 {
		return nil, err
	}
	return timestamps, nil
}

func extractAudio(videoPath string) (string, error) {
	dir := filepath.Dir(videoPath)
	base := filepath.Base(videoPath)
	audioFileName := strings.Replace(base, "video-", "audio-", 1)
	audioFileName = strings.Replace(audioFileName, ".mp4", ".mp3", 1)
	audioPath := filepath.Join(dir, audioFileName)

	cmd := exec.Command("ffmpeg", "-i", videoPath, "-vn", "-acodec", "mp3", audioPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to extract audio: %v", err)
	}

	if _, err := os.Stat(audioPath); err != nil {
		return "", fmt.Errorf("audio file was not created: %v", err)
	}

	return audioPath, nil
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
