package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/database"
)

func (cfg *apiConfig) generateAISmartResponse(w http.ResponseWriter, r *http.Request, thread string, transcriptDetails string) (openaiSmartResponse, error) {
	err := sendMessage(cfg.openaiClient, thread, transcriptDetails)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error sending message to openai", err.Error())
		return openaiSmartResponse{}, err
	}

	runID, err := getRunID(cfg.openaiClient, thread, cfg.assistantID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting run id", err.Error())
		return openaiSmartResponse{}, err
	}

	aiSmartResp, err := getResponse(cfg.openaiClient, thread, runID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting ai response", err.Error())
		return openaiSmartResponse{}, err
	}

	return aiSmartResp, nil
}

func (cfg *apiConfig) uploadAudioToS3(r *http.Request, voiceID, script, userID, seriesID string) (database.VideoSeries, error) {

	audioBytes, err := generateAudio(cfg, voiceID, script)
	if err != nil {
		return database.VideoSeries{}, err
	}
	fileName := fmt.Sprintf("audio_%s.mp4", uuid.New().String())
	tempDir := os.TempDir()
	safeFile, err := safePath(tempDir, fileName)
	if err != nil {
		return database.VideoSeries{}, err
	}

	err = os.WriteFile(safeFile, audioBytes, 0600)
	if err != nil {
		return database.VideoSeries{}, err
	}
	log.Println("audio file successfully saved: ", safeFile)
	defer os.Remove(safeFile)

	key, err := getAssestPath("audio/mp3")
	if err != nil {
		return database.VideoSeries{}, err
	}
	key = filepath.Join(userID, "test", "audio", key)

	processedFile, err := os.Open(safeFile)
	if err != nil {
		return database.VideoSeries{}, err
	}
	defer processedFile.Close()
	log.Println("opened processed")

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        processedFile,
		ContentType: aws.String("audio/mpeg"),
	})
	if err != nil {
		return database.VideoSeries{}, err
	}
	url := fmt.Sprintf("%s,%s", cfg.s3Bucket, key)
	series, err := cfg.db.SetAudioUrl(r.Context(), database.SetAudioUrlParams{
		AudioS3: url,
		ID:      seriesID,
	})
	if err != nil {
		return database.VideoSeries{}, err
	}

	series, err = cfg.dbAudioToSignedAudio(series)
	if err != nil {
		return database.VideoSeries{}, err
	}
	log.Println(series.AudioS3)

	return series, nil
}

func (cfg *apiConfig) handleVideoGeneration(w http.ResponseWriter, r *http.Request, avatarID string, series database.VideoSeries, user database.User, taskID string, aiResp openaiSmartResponse, video database.VideoTemplate) (string, error) {
	videoID, err := cfg.generateVideoHeygen(avatarID, series)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error generating video", err.Error())
		return "", err
	}

	heyUrl, err := cfg.getVideoStatus(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error downloading file", err.Error())
		return "", err
	}

	tempDir := os.TempDir()
	videoPath := filepath.Join(user.ID, taskID, "video.mp4")
	safeVideoPath, err := safePath(tempDir, videoPath)
	if err != nil {
		respondWithError(w, http.StatusIMUsed, "invalid video file path", err.Error())
		return "", err
	}
	defer os.Remove(safeVideoPath)

	if err := downloadHeygenVideo(heyUrl, safeVideoPath); err != nil {
		respondWithError(w, http.StatusInternalServerError, "error downloading video", err.Error())
		return "", err
	}

	audioPath, err := extractAudio(safeVideoPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error extracting audio", err.Error())
		return "", err
	}
	defer os.Remove(audioPath)

	timestamps, err := cfg.getCutTimestamps(audioPath, aiResp)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting timestamps", err.Error())
		return "", err
	}

	signedVid, err := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting signed video", err.Error())
		return "", err
	}

	templatePathDir := filepath.Join(user.ID, taskID)
	safeTemplatePathDir, err := safePath(tempDir, templatePathDir)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "invalid template directory path", err.Error())
		return "", err
	}

	templatePath, err := downloadFromS3(signedVid.S3Url.String, safeTemplatePathDir)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error downloading video template", err.Error())
		return "", err
	}
	defer os.Remove(templatePath)

	musicPath := filepath.Join("assets", "prelist", "prelist.mp3")
	return cfg.edit(safeVideoPath, audioPath, templatePath, musicPath, user.ID, timestamps)
}

func (cfg *apiConfig) uploadVideoToS3(w http.ResponseWriter, r *http.Request, videoPath, userID string, series database.VideoSeries) (database.VideoSeries, error) {
	const mediaType = "video/mp4"
	key, err := getAssestPath(mediaType)
	if err != nil {
		return database.VideoSeries{}, err
	}
	key = filepath.Join(userID, "fifs", key)
	proccessedFilePath, err := processVideoForFastStart(videoPath)
	if err != nil {
		return database.VideoSeries{}, err
	}

	defer os.Remove(proccessedFilePath)

	processedFile, err := os.Open(proccessedFilePath)
	if err != nil {
		return database.VideoSeries{}, err
	}

	log.Println("opened processed")
	defer processedFile.Close()

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        processedFile,
		ContentType: aws.String(mediaType),
	})
	if err != nil {
		return database.VideoSeries{}, err
	}

	log.Println("UPLOADED FIF TO S3")

	url := fmt.Sprintf("%s,%s", cfg.s3Bucket, key)
	series.S3Url = sql.NullString{String: url, Valid: url != ""}
	seriesWithFIF, err := cfg.db.SetFIFUrl(r.Context(), database.SetFIFUrlParams{
		ID:    series.ID,
		S3Url: series.S3Url,
	})
	if err != nil {
		return database.VideoSeries{}, err
	}

	return seriesWithFIF, nil
}
