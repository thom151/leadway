// #nosec G204 G304

package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/database"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid id", err.Error())
		return
	}

	log.Println("go video id")

	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid user", err.Error())
		return
	}

	video, err := cfg.db.GetVideoById(r.Context(), videoID.String())

	if video.UserID != user.ID {
		respondWithError(w, http.StatusUnauthorized, "unauthorized access", err.Error())
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "cannot find video", err.Error())
		return
	}
	defer file.Close()
	log.Println("form file")

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid content-type", err.Error())
		return
	}

	log.Println("media-type", mediaType)

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "invalid media type", err.Error())
		return
	}

	tempDir := os.TempDir()
	videoFile, err := os.CreateTemp(tempDir, "leadway-template-upload-*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error saving file", err.Error())
		return
	}
	defer os.Remove(videoFile.Name())
	defer videoFile.Close()
	log.Println("created temp successful")

	_, err = io.Copy(videoFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error copying file", err.Error())
		return
	}
	log.Println("copy successful")

	transcodedFilePath := fmt.Sprintf("%s-transcoded.mp4", videoFile.Name())
	//nolint:gosec // G204: safePath-validated
	cmd := exec.Command("ffmpeg", "-i", videoFile.Name(), "-c:v", "libx264", "-c:a", "aac", "-f", "mp4", transcodedFilePath)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout // Log FFmpeg output
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "error transcoding video to H.264", fmt.Sprintf("%v, stderr: %s", err, stderr.String()))
		return
	}
	defer os.Remove(transcodedFilePath)
	log.Println("transcoded video to H.264")

	key, err := getAssestPath(mediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating key", err.Error())
		return
	}

	key = filepath.Join(user.ID, "templates", key)

	processedFilePath, err := processVideoForFastStart(transcodedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error processing fast video", err.Error())
		return
	}
	defer os.Remove(processedFilePath)
	log.Println("processed video file:", processedFilePath)
	//nolint:gosec // G304: safePath-validated
	processedFile, err := os.Open(processedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error opening fast video file", err.Error())
		return
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
		respondWithError(w, http.StatusInternalServerError, "error uploading file to s3", err.Error())
		return
	}

	log.Println("s3 successful")

	url := fmt.Sprintf("%s,%s", cfg.s3Bucket, key)
	video.S3Url = sql.NullString{String: url, Valid: url != ""}
	err = cfg.db.UpdateVideo(r.Context(), database.UpdateVideoParams{
		Title:       video.Title,
		Description: video.Description,
		S3Url:       video.S3Url,
		UserID:      video.UserID,
		ID:          video.ID,
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video", err.Error())
		return
	}

	video, err = cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error generating presigned url", err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
