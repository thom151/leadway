// #nosec G204 G304
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

type BrollPathSet struct {
	B1 string
	B2 string
	B3 string
	B4 string
}

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
	fileName := fmt.Sprintf("audio_%s.mp3", uuid.New().String())
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

func (cfg *apiConfig) handleVideoGeneration(w http.ResponseWriter, r *http.Request, avatarID string, series database.VideoSeries, user database.User, taskID string, aiResp openaiSmartResponse, bSet BrollSet) (string, error) {
	tempDir := os.TempDir()

	videoID, err := cfg.generateVideoHeygen(avatarID, series)
	if err != nil {
		log.Println("error generating video")
		//	respondWithError(w, http.StatusInternalServerError, "error generating video", err.Error())
		return "", err
	}

	heyUrl, err := cfg.getVideoStatus(videoID)
	if err != nil {
		log.Println("error  downloading file")
		////	respondWithError(w, http.StatusInternalServerError, "error downloading file", err.Error())
		return "", err
	}

	videoPath := filepath.Join(user.ID, taskID, "video.mp4")
	safeVideoPath, err := safePath(tempDir, videoPath)
	if err != nil {
		log.Println("invalid video file path")
		////	respondWithError(w, http.StatusIMUsed, "invalid video file path", err.Error())
		return "", err
	}
	defer os.Remove(safeVideoPath)

	if err := downloadHeygenVideo(heyUrl, safeVideoPath); err != nil {
		log.Println("error download hey video")
		////	respondWithError(w, http.StatusInternalServerError, "error downloading video", err.Error())
		return "", err
	}

	audioPath, err := extractAudio(safeVideoPath)
	if err != nil {
		log.Println("error extracting audio")
		//	respondWithError(w, http.StatusInternalServerError, "error extracting audio", err.Error())
		return "", err
	}
	defer os.Remove(audioPath)

	timestamps, err := cfg.getCutTimestamps(audioPath, aiResp)
	if err != nil {
		log.Println("error getting timestamps")
		////	respondWithError(w, http.StatusInternalServerError, "error getting timestamps", err.Error())
		return "", err
	}

	/*
		signedVid, err := cfg.dbVideoToSignedVideo(bSet.B1)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "error getting signed broll1", err.Error())
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

	*/
	brollPaths, err := cfg.getBrollPaths(w, bSet, user.ID, taskID, tempDir)
	if err != nil {
		log.Printf("error getting broll paths: %v\n", err)
		////	respondWithError(w, http.StatusInternalServerError, "error getting broll paths", err.Error())
		return "", err
	}
	log.Println("got boll paths")

	for _, p := range []string{brollPaths.B1, brollPaths.B2, brollPaths.B3, brollPaths.B4} {
		defer os.Remove(p)
	}

	musicPath := filepath.Join("assets", "prelist", "prelist.mp3")

	return cfg.edit2(safeVideoPath, audioPath, musicPath, user.ID, brollPaths, timestamps)
}

func (cfg *apiConfig) getBrollPaths(w http.ResponseWriter, bSet BrollSet, userID, taskID, tempDir string) (BrollPathSet, error) {
	var result BrollPathSet
	brolls := []struct {
		video database.VideoTemplate
		label string
		dest  *string
	}{
		{bSet.B1, "brollTmp1", &result.B1},
		{bSet.B2, "brollTmp2", &result.B2},
		{bSet.B3, "brollTmp3", &result.B3},
		{bSet.B4, "brollTmp4", &result.B4},
	}

	for _, b := range brolls {
		signedVid, err := cfg.dbVideoToSignedVideo(b.video)
		if err != nil {
			//respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("error getting signed %s", b.label), err.Error())

			return BrollPathSet{}, err
		}

		downloadDir := filepath.Join(userID, taskID)
		safeDir, err := safePath(tempDir, downloadDir)
		if err != nil {
			//respondWithError(w, http.StatusInternalServerError, "invalid template directory path", err.Error())
			return BrollPathSet{}, err
		}
		log.Printf("Downloading %s from: %s", b.label, signedVid.S3Url.String)

		fileName := fmt.Sprintf("%s.mp4", b.label)
		savePath := filepath.Join(safeDir, fileName)
		path, err := downloadFromS3(signedVid.S3Url.String, savePath)
		if err != nil {
			//respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("error downloading %s", b.label), err.Error())
			return BrollPathSet{}, err
		}

		*b.dest = path
	}

	return result, nil
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
