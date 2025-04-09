// #nosec G204 G304

package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func downloadFromS3(s3URL, taskPath string) (string, error) {
	fullBrollPath := filepath.Join(taskPath, "template.mp4")
	bucket, key, err := parseS3URL(s3URL)
	if err != nil {
		return "", err
	}
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to load s3 config: %v", err)
	}
	client := s3.NewFromConfig(cfg)
	resp, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := os.MkdirAll(taskPath, 0750); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %v", taskPath, err)
	}

	//nolint:gosec // G304: safePath-validated
	f, err := os.OpenFile(fullBrollPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %v", fullBrollPath, err)
	}
	defer f.Close()

	if _, err = io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write S3 content to file %s: %v", fullBrollPath, err)
	}

	if _, err := os.Stat(fullBrollPath); err != nil {
		return "", fmt.Errorf("file %s was not created successfully: %v", fullBrollPath, err)
	}
	return fullBrollPath, nil
}

func parseS3URL(s3URL string) (bucket, key string, err error) {
	u, err := url.Parse(s3URL)
	if err != nil {
		return "", "", err
	}
	if !strings.Contains(u.Host, ".s3") || !strings.HasSuffix(u.Host, ".amazonaws.com") {
		return "", "", fmt.Errorf("invalid s3 Url : %s", s3URL)
	}
	hostParts := strings.Split(u.Host, ".")
	if len(hostParts) < 4 {
		return "", "", fmt.Errorf("invalid s3 Url: %s", s3URL)
	}
	bucket = hostParts[0]
	key = strings.TrimPrefix(u.Path, "/")
	if key == "" {
		return "", "", fmt.Errorf("s3 key is empy in url: %s", s3URL)
	}

	return bucket, key, nil
}
