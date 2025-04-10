// #nosec G204 G304

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/database"
)

type genVideoFastParams struct {
	VideoID       string `json:"video_id"`
	ClientName    string `json:"client_name"`
	ClientAddress string `json:"client_address"`
	Personalized  string `json:"personalized"`
	SeriesID      string `json:"series_id"`
}

func (cfg *apiConfig) handlerGenerateVideo(w http.ResponseWriter, r *http.Request) {
	taskID := uuid.New().String()
	avatarID := r.PathValue("avatarID")
	if avatarID == "" {
		respondWithError(w, http.StatusBadRequest, "missing avatar id", "please provide an avatar id")
		return
	}

	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid user", err.Error())
		return
	}
	decoder := json.NewDecoder(r.Body)
	var params genVideoFastParams
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "cannot decode parameters", err.Error())
		return
	}

	log.Println("VIDEO ID: ", params.VideoID)
	video, err := cfg.db.GetVideoById(r.Context(), params.VideoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting video", err.Error())
		return
	}
	series, err := cfg.db.GetVideoSeriesById(r.Context(), params.SeriesID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting series id", err.Error())
		return
	}

	threadID, err := genThread(cfg.openaiClient, user.Username)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error generating thread", err.Error())
		return
	}

	transcript_details := fmt.Sprintf("Agent Name: %s, Client name: %s, Client Address: %s, Other Details: %s", user.Username, params.ClientName, params.ClientAddress, series.Description.String)
	fmt.Println("Transcript details: ", transcript_details)

	aiSmartResp, err := cfg.generateAISmartResponse(w, r, threadID.ID, transcript_details)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error generating ai response", err.Error())
		return
	}
	seriesWithAudio, err := cfg.uploadAudioToS3(r, "EOdiXIQ9NErAFc1UUoEH", aiSmartResp.FullScript, user.ID, series.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading audio to s3", err.Error())
		return
	}

	outputPath, err := cfg.handleVideoGeneration(w, r, avatarID, seriesWithAudio, user, taskID, aiSmartResp, video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error handling video generation", err.Error())
		return
	}

	seriesWithFIF, err := cfg.uploadVideoToS3(w, r, outputPath, user.ID, seriesWithAudio)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading fif to s3", err.Error())
		return
	}

	signedFIF, err := cfg.dbFIFToSignedFIF(seriesWithFIF)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting signed fif", err.Error())
		return
	}
	log.Println("FIF: ", signedFIF.S3Url.String)
	log.Println("Successfully edited: ", outputPath)

	respondWithJSON(w, http.StatusOK, "ok")

}

func (cfg *apiConfig) dbAudioToSignedAudio(series database.VideoSeries) (database.VideoSeries, error) {
	if series.AudioS3 == "unset" {
		return series, nil
	}
	parts := strings.Split(series.AudioS3, ",")
	if len(parts) < 2 {
		return series, nil
	}
	bucket := parts[0]
	key := parts[1]
	presigned, err := generatePresignedURL(cfg.s3Client, bucket, key, 5*time.Minute)
	if err != nil {
		return series, err
	}
	series.AudioS3 = presigned
	return series, nil

}

func (cfg *apiConfig) dbFIFToSignedFIF(series database.VideoSeries) (database.VideoSeries, error) {
	if series.S3Url.String == "" {
		return series, nil
	}
	parts := strings.Split(series.S3Url.String, ",")
	if len(parts) < 2 {
		return series, nil
	}
	bucket := parts[0]
	key := parts[1]
	presigned, err := generatePresignedURL(cfg.s3Client, bucket, key, 5*time.Minute)
	if err != nil {
		return series, err
	}
	series.S3Url.String = presigned
	return series, nil

}
