package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/database"
)

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
	var params genVideoParams
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "cannot decode parameters", err.Error())
		return
	}
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

	thread, err := cfg.db.GetThread(r.Context(), database.GetThreadParams{
		UserID:    user.ID,
		ContactID: "0413810844",
	})
	if err != nil {
		threadID, err := genThread(cfg.openaiClient, user.Username)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "error generating thread", err.Error())
			return
		}

		thread, err = cfg.db.CreateThread(r.Context(), database.CreateThreadParams{
			UserID:    user.ID,
			ContactID: user.Email,
			ThreadID:  threadID.ID,
		})
	}
	log.Println("Got Thread \n")

	transcript_details := fmt.Sprintf("Agent Name: %s, Client name: %s, Client Address: %s, Other Details: %s", user.Username, client.Name, client.Address.String, series.Description.String)
	fmt.Println("Transcript details: ", transcript_details)

	err = sendMessage(cfg.openaiClient, thread.ThreadID, transcript_details)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error sending message to openai", err.Error())
		return
	}

	runID, err := getRunID(cfg.openaiClient, thread.ThreadID, cfg.assistantID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting run id", err.Error())
		return
	}

	aiSmartResp, err := getResponse(cfg.openaiClient, thread.ThreadID, runID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting ai response", err.Error())
		return
	}

	audioBytes, err := generateAudio(cfg, "EOdiXIQ9NErAFc1UUoEH", aiSmartResp.FullScript)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error generating audio", err.Error())
		return
	}
	log.Println("generated audio successfuly\n")
	audioFile := fmt.Sprintf("audio_%s.mp3", uuid.New().String())
	err = os.WriteFile(audioFile, audioBytes, 0644)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error saving audio file", err.Error())
		return
	}

	log.Println("audio file successfully saved\n")

	defer os.Remove(audioFile)

	key, err := getAssestPath("audio/mp3")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting path", err.Error())
		return
	}
	key = filepath.Join(user.ID, client.ID, "audio", key)

	processedFile, err := os.Open(audioFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error opening fast video", err.Error())
		return
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
		respondWithError(w, http.StatusInternalServerError, "error uploading file to s3", err.Error())
		return
	}

	url := fmt.Sprintf("%s,%s", cfg.s3Bucket, key)
	_, err = cfg.db.SetAudioUrl(r.Context(), database.SetAudioUrlParams{
		AudioS3: url,
		ID:      series.ID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video series", err.Error())
		return
	}

	series, err = cfg.dbAudioToSignedAudio(series)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error generating presigned url", err.Error())
		return
	}
	time.Sleep(10 * time.Second)
	log.Println(series.AudioS3)

	log.Println("generating video... \n\n")
	videoID, err := cfg.generateVideoHeygen(avatarID, series)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error generating video", err.Error())
		return
	}
	log.Println("Video ID: ", videoID)
	log.Println("downloading video... \n\n")

	heyUrl, err := cfg.getVideoStatus(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error downloading file", err.Error())
		return
	}
	videoPath := filepath.Join("temp", user.ID, taskID, "video.mp4")
	err = downloadHeygenVideo(heyUrl, videoPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error extracting audio", err.Error())
		return
	}

	log.Println("video downloaded")
	audioPath, err := extractAudio(videoPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error extracting audio", err.Error())
		return
	}

	timestamps, err := cfg.getCutTimestamps(audioPath, aiSmartResp)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting timestamps", err.Error())
		return
	}

	log.Println("TIMESTAMPS: ", timestamps[0], ", ", timestamps[1])
	signedVid, err := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error gettign signed video", err.Error())
		return
	}

	templatePath, err := downloadFromS3(signedVid.S3Url.String, filepath.Join("temp", user.ID, taskID))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error downloading video template", err.Error())
		return
	}
	outputFile, err := cfg.edit(videoPath, audioPath, templatePath, user.ID, timestamps)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error editing the video", err.Error())
		return
	}

	log.Println("Successfully edited: ", outputFile)

	respondWithJSON(w, http.StatusOK, "ok")

}
