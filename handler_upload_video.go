package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	//	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
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

	videoFile, err := os.CreateTemp("", "leadway-template-upload.mp4")
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
	cmd := exec.Command("ffmpeg", "-i", videoFile.Name(), "-c:v", "libx264", "-c:a", "aac", "-f", "mp4", transcodedFilePath)
	cmd.Stdout = os.Stdout // Log FFmpeg output
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "error transcoding video to H.264", err.Error())
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
	log.Println("processed video file")

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

func getAssestPath(mediaType string) (string, error) {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		log.Println("error reading random bytes")
		return "", err
	}

	randString := base64.RawURLEncoding.EncodeToString(randomBytes)

	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid media type")
	}
	ext := "." + parts[1]
	return fmt.Sprintf("%s%s", randString, ext), nil

}

func (cfg *apiConfig) getS3Url(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
}

func processVideoForFastStart(inputFilePath string) (string, error) {
	processedFilePath := fmt.Sprintf("%s.processing", inputFilePath)

	cmd := exec.Command("ffmpeg", "-i", inputFilePath, "-movflags", "faststart", "-codec", "copy", "-f", "mp4", processedFilePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error processing video: %s, %v", stderr.String(), err)
	}

	fileInfo, err := os.Stat(processedFilePath)
	if err != nil {
		return "", fmt.Errorf("could not stat processed file: %v", err)
	}
	if fileInfo.Size() == 0 {
		return "", fmt.Errorf("processed file is empty")
	}

	return processedFilePath, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.VideoTemplate) (database.VideoTemplate, error) {
	if video.S3Url.String == "" {
		return video, nil
	}
	parts := strings.Split(video.S3Url.String, ",")
	if len(parts) < 2 {
		return video, nil
	}
	bucket := parts[0]
	key := parts[1]
	presigned, err := generatePresignedURL(cfg.s3Client, bucket, key, 5*time.Minute)
	if err != nil {
		return video, err
	}
	video.S3Url = sql.NullString{String: presigned, Valid: presigned != ""}
	return video, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)
	presignedUrl, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %v", err)
	}
	return presignedUrl.URL, nil
}
