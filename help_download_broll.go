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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/thom151/leadme/internal/database"
)

type BrollSet struct {
	B1 database.VideoTemplate
	B2 database.VideoTemplate
	B3 database.VideoTemplate
	B4 database.VideoTemplate
}

func downloadFromS3(s3URL, fullPath string) (string, error) {
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

	if err := os.MkdirAll(filepath.Dir(fullPath), 0750); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %v", fullPath, err)
	}

	//nolint:gosec // G304: safePath-validated
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %v", fullPath, err)
	}
	defer f.Close()

	if _, err = io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write S3 content to file %s: %v", fullPath, err)
	}

	if _, err := os.Stat(fullPath); err != nil {
		return "", fmt.Errorf("file %s was not created successfully: %v", fullPath, err)
	}
	return fullPath, nil
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

func (cfg *apiConfig) getBrolls(b1Str, b2Str, b3Str, b4Str string) (set BrollSet, err error) {

	broll1, err := cfg.db.GetVideoById(context.Background(), b1Str)
	if err != nil {
		return BrollSet{}, err
	}

	broll2, err := cfg.db.GetVideoById(context.Background(), b2Str)
	if err != nil {

		return BrollSet{}, err
	}

	broll3, err := cfg.db.GetVideoById(context.Background(), b3Str)
	if err != nil {

		return BrollSet{}, err
	}

	broll4, err := cfg.db.GetVideoById(context.Background(), b4Str)
	if err != nil {
		return BrollSet{}, err
	}

	return BrollSet{
		B1: broll1,
		B2: broll2,
		B3: broll3,
		B4: broll4,
	}, nil

}

func (cfg *apiConfig) downloadOriginalAudioFromS3(ctx context.Context, region, bucket, key, outPath string) error {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.s3Region))
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)

	obj, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}
	defer obj.Body.Close()

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	defer f.Close()

	if _, err := io.Copy(f, obj.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}
