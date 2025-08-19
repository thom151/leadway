// #nosec G204 G304

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	//	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/database"
)

type genVideoFastParams struct {
	Broll1ID      string `json:"broll1_id"`
	Broll2ID      string `json:"broll2_id"`
	Broll3ID      string `json:"broll3_id"`
	Broll4ID      string `json:"broll4_id"`
	AgentName     string `json:"agent_name"`
	ClientName    string `json:"client_name"`
	ClientAddress string `json:"client_address"`
	Personalized  string `json:"personalized"`
	SeriesID      string `json:"series_id"`
	VoiceID       string `json:"voice_id"`
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

	//validate if user can access avatarID

	decoder := json.NewDecoder(r.Body)
	var params genVideoFastParams
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "cannot decode parameters", err.Error())
		return
	}

	log.Println("VIDEO ID: ", params.Broll1ID)
	brollSet, err := cfg.getBrolls(params.Broll1ID, params.Broll2ID, params.Broll3ID, params.Broll4ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting video", err.Error())
		return
	}

	//validate if user can access videoID/broll

	series, err := cfg.db.GetVideoSeriesById(r.Context(), params.SeriesID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting series id", err.Error())
		return
	}

	//validate if user can access series

	threadID, err := genThread(cfg.openaiClient, user.Username)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error generating thread", err.Error())
		return
	}

	transcript_details := fmt.Sprintf("Agent Name: %s, Client name: %s, Client Address: %s, Other Details: %s", params.AgentName, params.ClientName, params.ClientAddress, series.Description.String)
	fmt.Println("Transcript details: ", transcript_details)

	aiSmartResp, err := cfg.generateAISmartResponse(w, r, threadID.ID, transcript_details)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error generating ai response", err.Error())
		return
	}

	//THIS IS WHAT WE WANT TO CHANGE

	//get the original audio from s3
	//combine that with the generated audio
	//
	seriesWithAudio, err := cfg.uploadAudioToS3(r, params.VoiceID, aiSmartResp.FullScript, user.ID, series.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading audio to s3", err.Error())
		return
	}

	//THIS IS WHERE EDITING HAPPENS
	outputPath, err := cfg.handleVideoGeneration(w, r, avatarID, seriesWithAudio, user, taskID, aiSmartResp, brollSet)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error handling video generation", err.Error())
		return
	}
	//defer os.Remove(outputPath)

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

	//http.Redirect(w, r, fmt.Sprintf("/fif/%s", seriesWithFIF.ID), http.StatusSeeOther)
	respondWithJSON(w, http.StatusOK, signedFIF)
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
	presigned, err := generatePresignedURL(cfg.s3Client, bucket, key, 168*time.Hour)
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
	presigned, err := generatePresignedURL(cfg.s3Client, bucket, key, 7*24*time.Hour)
	if err != nil {
		return series, err
	}
	series.S3Url.String = presigned
	return series, nil

}
