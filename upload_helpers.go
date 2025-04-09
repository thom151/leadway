package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/thom151/leadme/internal/database"
)

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

/*
func (cfg *apiConfig) getS3Url(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
}
*/

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
