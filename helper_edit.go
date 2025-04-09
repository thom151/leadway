package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type videoSeriesFormat struct {
	VideoCodec    string
	AudioCodec    string
	FrameRate     string
	PixelFormat   string
	SampleRate    string
	ChannelLayout string
}

func (cfg *apiConfig) edit(video, audio, broll, music, userId string, ts []float64) (string, error) {
	var filesForCleanup []string

	taskPath := filepath.Dir(video)
	audioTaskPath := filepath.Dir(audio)
	if taskPath != audioTaskPath {
		return "", fmt.Errorf("video and audio file not in the same directory")
	}
	totalDuration, err := getTotalDuration(video)
	if err != nil {
		return "", err
	}

	brollDuration := ts[1] - ts[0]

	if ts[1] > totalDuration || ts[0] > ts[1] {
		return "", fmt.Errorf("error with timestamps and durations")
	}

	videoFormat := videoSeriesFormat{
		VideoCodec:    "libx264",
		AudioCodec:    "aac",
		FrameRate:     "30",
		PixelFormat:   "yuv420p",
		SampleRate:    "44100",
		ChannelLayout: "stereo",
	}

	segment1 := filepath.Join(taskPath, "segment1.mp4")
	segment2 := filepath.Join(taskPath, "segment2.mp4")
	segment3 := filepath.Join(taskPath, "segment3.mp4")
	musicOut := filepath.Join(taskPath, "music.mp4")

	err = cutAndSaveVideo(video, segment1, 0, ts[0], videoFormat)
	if err != nil {
		return "", err
	}
	log.Println("SEGMENT 1 SAVED ... ")
	err = cutAndSaveVideo(broll, segment2, 0, brollDuration, videoFormat, true)
	if err != nil {
		return "", err
	}
	log.Println("SEGMENT 2 SAVED ... ")
	err = cutAndSaveVideo(video, segment3, ts[1], totalDuration-ts[1], videoFormat)
	if err != nil {
		return "", err
	}
	log.Println("SEGMENT 3 SAVED ... ")
	err = cutAndSaveAudio(music, musicOut, totalDuration, videoFormat)
	if err != nil {
		return "", err
	}
	log.Println("MUSIC SAVED ... ")
	filesForCleanup = append(filesForCleanup, segment1, segment2, segment3, musicOut)

	//put each segment in a file
	concatTextFile := filepath.Join(taskPath, "concat.txt")
	f, err := os.OpenFile(concatTextFile, os.O_CREATE|os.O_WRONLY, 0600) // Use 0600 permissions
	if err != nil {
		return "", fmt.Errorf("failed to create concat.txt: %v", err)
	}
	defer f.Close()
	concatContent := strings.NewReader(fmt.Sprintf("file '%s'\nfile '%s'\nfile '%s'\n",
		filepath.Base(segment1), filepath.Base(segment2), filepath.Base(segment3)))
	if _, err := io.Copy(f, concatContent); err != nil {
		return "", fmt.Errorf("failed to write concat.txt: %v", err)
	}
	filesForCleanup = append(filesForCleanup, concatTextFile)
	log.Println("CONCAT TEXT FILE SAVED ... ")
	// use ffmpeg to concatenate all three files
	concatOutputPath, err := concatVideosFromTextFile(concatTextFile, taskPath, videoFormat)
	if err != nil {
		return "", err
	}
	filesForCleanup = append(filesForCleanup, concatOutputPath)
	log.Println("CONCATENATED VIDEO SAVED ...")
	//overlay audio
	finalVideoOutputPath, err := overlayAudio(audio, musicOut, concatOutputPath, taskPath, videoFormat)
	if err != nil {
		return "", err
	}
	log.Println("Final Video Saved ...")
	err = cleanFiles(filesForCleanup)
	if err != nil {
		return "", err
	}

	return finalVideoOutputPath, nil
}

func cleanFiles(files []string) error {
	var errs []string
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			errs = append(errs, fmt.Sprintf("failed to remove %s: %v", file, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func overlayAudio(audioPath, musicPath, concatenatedPath, taskPath string, videoFormat videoSeriesFormat) (string, error) {
	outputPath := filepath.Join(taskPath, "output.mp4")

	cmd := exec.Command("ffmpeg",
		"-i", concatenatedPath,
		"-i", audioPath,
		"-i", musicPath,
		"-filter_complex",
		"[1:a]volume=1.5[a1];[2:a]volume=0.3[a2];[a1][a2]amix=inputs=2:duration=first:dropout_transition=0[a]",
		"-map", "0:v:0",
		"-map", "[a]",
		"-c:v", "copy",
		"-c:a", videoFormat.AudioCodec,
		"-ar", videoFormat.SampleRate,
		"-channel_layout", videoFormat.ChannelLayout,
		"-movflags", "faststart",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to overlay audio: %v, stderr: %s", err, stderr.String())
	}
	return outputPath, nil
}

func concatVideosFromTextFile(textFile, taskPath string, videoFormat videoSeriesFormat) (string, error) {
	concatOutput := filepath.Join(taskPath, "concat_video.mp4")

	cmd := exec.Command("ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-i", textFile,
		"-c:v", videoFormat.VideoCodec,
		"-preset", "fast",
		"-an",
		"-r", videoFormat.FrameRate,
		"-pix_fmt", videoFormat.PixelFormat,
		concatOutput,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err

	}
	return concatOutput, nil
}

func cutAndSaveAudio(audioPath, outputPath string, duration float64, videoFormat videoSeriesFormat) error {
	totalDuration, err := getTotalDuration(audioPath)
	if err != nil {
		return err
	}

	if totalDuration < duration {
		return fmt.Errorf("requested duration exceeds music duration")
	}

	fadeDuration := 2.0
	fadeStart := duration - fadeDuration
	args := []string{
		"-i", audioPath,
		"-t", fmt.Sprintf("%.2f", duration), // Duration to cut
		"-af", fmt.Sprintf("afade=t=out:st=%.2f:d=%.2f", fadeStart, fadeDuration), // Fade-out filter
		"-c:a", videoFormat.AudioCodec,
		"-ar", videoFormat.SampleRate,
		"-channel_layout", videoFormat.ChannelLayout,
		outputPath,
	}
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	log.Printf("Running FFmpeg command: %v", cmd.Args)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to cut audio: %v, stderr: %s", err, stderr.String())
	}

	if _, err := os.Stat(outputPath); err != nil {
		return fmt.Errorf("output audio file was not created: %v", err)
	}
	outputDuration, err := getTotalDuration(outputPath)
	if err != nil {
		return fmt.Errorf("output audio file %s is not playable: %v", outputPath, err)
	}
	log.Printf("Output audio file %s created with duration %.2f seconds", outputPath, outputDuration)

	return nil
}

func getTotalDuration(video string) (float64, error) {
	var out, stderr bytes.Buffer
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", video)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("failed to get video duration: %v, stderr: %s", err, stderr.String())
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(out.String()), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %v", err)
	}
	return duration, nil
}

func cutAndSaveVideo(inputPath, outputPath string, startTime, duration float64, videoFormat videoSeriesFormat, muteAudio ...bool) error {
	args := []string{
		"-ss", fmt.Sprintf("%.2f", startTime),
		"-i", inputPath,
		"-t", fmt.Sprintf("%.2f", duration),
		"-c:v", videoFormat.VideoCodec,
		"-preset", "fast",
		"-r", videoFormat.FrameRate,
		"-pix_fmt", videoFormat.PixelFormat,
	}

	if len(muteAudio) == 0 || !muteAudio[0] {
		args = append(args,
			"-c:a", videoFormat.AudioCodec,
			"-ar", videoFormat.SampleRate,
			"-channel_layout", videoFormat.ChannelLayout,
		)
	} else {
		args = append(args, "-an") // Mute audio
	}

	var stderr bytes.Buffer
	args = append(args, "-f", "mp4", "-movflags", "faststart", outputPath)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = &stderr

	// Log the FFmpeg command for debugging
	log.Printf("Running FFmpeg command: %v", cmd.Args)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run FFmpeg: %v, stderr: %s", err, stderr.String())
	}
	if _, err := os.Stat(outputPath); err != nil {
		return fmt.Errorf("output file was not created: %v", err)
	}
	return nil

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

	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("error creating directory %s: %w", dir, err)
	}

	cmd := exec.Command("ffmpeg", "-i", videoPath, "-vn", "-acodec", "mp3", audioPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	log.Printf("Running FFmpeg command: %v", cmd.Args)
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
