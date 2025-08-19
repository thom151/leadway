package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func (cfg *apiConfig) edit2(video, audio, music, userId string, brollPaths BrollPathSet, ts []float64) (string, error) {
	var filesForCleanup []string

	videoFormat := videoSeriesFormat{
		VideoCodec:    "libx264",
		AudioCodec:    "aac",
		FrameRate:     "30",
		PixelFormat:   "yuv420p",
		SampleRate:    "44100",
		ChannelLayout: "stereo",
	}

	taskPath := filepath.Dir(video)
	audioTaskPath := filepath.Dir(audio)
	if taskPath != audioTaskPath {
		return "", fmt.Errorf("video and audio file not in the same directory")
	}

	// Validate timestamps
	if len(ts) != 7 {
		return "", fmt.Errorf("expected 7 timestamps, got %d", len(ts))
	}

	totalDuration, err := getTotalDuration(video)
	outroDuration, err := getTotalDuration(brollPaths.B4)

	if err != nil {
		return "", err
	}

	if ts[6] > totalDuration {
		return "", fmt.Errorf("last timestamp exceeds video duration")
	}

	// Durations
	agent1Duration := ts[0]
	broll1Duration := ts[1] - ts[0]
	agent2Duration := ts[2] - ts[1]
	broll2Duration := ts[3] - ts[2]
	agent3Duration := ts[4] - ts[3]
	broll3Duration := ts[5] - ts[4]
	agent4Duration := ts[6] - ts[5]

	// Segment paths
	segmentPaths := []string{
		filepath.Join(taskPath, "agent1.mp4"),
		filepath.Join(taskPath, "broll1.mp4"),
		filepath.Join(taskPath, "agent2.mp4"),
		filepath.Join(taskPath, "broll2.mp4"),
		filepath.Join(taskPath, "agent3.mp4"),
		filepath.Join(taskPath, "broll3.mp4"),
		filepath.Join(taskPath, "agent4.mp4"),
		filepath.Join(taskPath, "broll4.mp4"),
	}

	// Cut each segment
	err = cutAndSaveVideo(video, segmentPaths[0], 0, agent1Duration, videoFormat)
	if err != nil {
		return "", fmt.Errorf("failed to cut agent1: %w", err)
	}

	err = cutAndSaveVideo(brollPaths.B1, segmentPaths[1], 0, broll1Duration, videoFormat, true)
	if err != nil {
		return "", fmt.Errorf("failed to cut broll1: %w", err)
	}

	err = cutAndSaveVideo(video, segmentPaths[2], ts[1], agent2Duration, videoFormat)
	if err != nil {
		return "", fmt.Errorf("failed to cut agent2: %w", err)
	}

	err = cutAndSaveVideo(brollPaths.B2, segmentPaths[3], 0, broll2Duration, videoFormat, true)
	if err != nil {
		return "", fmt.Errorf("failed to cut broll2: %w", err)
	}

	err = cutAndSaveVideo(video, segmentPaths[4], ts[3], agent3Duration, videoFormat)
	if err != nil {
		return "", fmt.Errorf("failed to cut agent3: %w", err)
	}

	err = cutAndSaveVideo(brollPaths.B3, segmentPaths[5], 0, broll3Duration, videoFormat, true)
	if err != nil {
		return "", fmt.Errorf("failed to cut broll3: %w", err)
	}

	err = cutAndSaveVideo(video, segmentPaths[6], ts[5], agent4Duration, videoFormat)
	if err != nil {
		return "", fmt.Errorf("failed to cut agent4: %w", err)
	}

	err = cutAndSaveVideo(brollPaths.B4, segmentPaths[7], 0, outroDuration, videoFormat, true)
	if err != nil {
		return "", fmt.Errorf("failed to cut broll4: %w", err)
	}

	log.Println("All segments cut successfully")
	filesForCleanup = append(filesForCleanup, segmentPaths...)

	broll4ActualDuration, err := getTotalDuration(segmentPaths[7])
	if err != nil {
		return "", fmt.Errorf("failed to get duration of full broll4: %w", err)
	}

	totalDuration = ts[6] + broll4ActualDuration
	// Cut background music
	musicOut := filepath.Join(taskPath, "music.mp4")
	err = cutAndSaveAudio(music, musicOut, totalDuration, videoFormat)
	if err != nil {
		fmt.Errorf("failed to cut background music: %w", err)
		return "", fmt.Errorf("failed to cut background music: %w", err)
	}
	filesForCleanup = append(filesForCleanup, musicOut)
	log.Println("cut and saved audio")

	// Create concat.txt
	concatTextFile := filepath.Join(taskPath, "concat.txt")
	f, err := os.OpenFile(concatTextFile, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Errorf("failed to create concat.txt: %w", err)
		return "", fmt.Errorf("failed to create concat.txt: %w", err)
	}
	defer f.Close()

	var concatBuilder strings.Builder
	for _, segment := range segmentPaths {
		concatBuilder.WriteString(fmt.Sprintf("file '%s'\n", filepath.Base(segment)))
	}
	if _, err := f.WriteString(concatBuilder.String()); err != nil {
		return "", fmt.Errorf("failed to write to concat.txt: %w", err)
	}
	filesForCleanup = append(filesForCleanup, concatTextFile)

	// Concatenate all segments
	concatOutputPath, err := concatVideosFromTextFile(concatTextFile, taskPath, videoFormat)
	if err != nil {
		return "", fmt.Errorf("failed to concatenate video segments: %w", err)
	}
	filesForCleanup = append(filesForCleanup, concatOutputPath)

	// Overlay voice + music
	finalVideoOutputPath, err := overlayAudio(audio, musicOut, concatOutputPath, taskPath, videoFormat)
	if err != nil {
		return "", fmt.Errorf("failed to overlay audio: %w", err)
	}

	log.Println("Final video saved:", finalVideoOutputPath)

	// Cleanup
	if err := cleanFiles(filesForCleanup); err != nil {
		return "", fmt.Errorf("cleanup error: %w", err)
	}

	return finalVideoOutputPath, nil

}
