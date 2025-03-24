package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

type genVideoParams struct {
	VideoID      string `json:"video_id"`
	ClientID     string `json:"client_id"`
	Personalized string `json:"personalized"`
	SeriesID     string `json:"series_id"`
}

type CutWord struct {
	Word  string `json:"word"`
	Index int    `json:"index"`
}

type openaiSmartResponse struct {
	FullScript string    `json:"full_script"`
	CutWords   []CutWord `json:"cut_words"`
}

func (cfg *apiConfig) handlerGenerateVideoSeries(w http.ResponseWriter, r *http.Request) {
	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid user", err.Error())
		return
	}
	/*
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

			client, err := cfg.db.GetClientById(r.Context(), params.ClientID)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "error getting client", err.Error())
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


				audioBytes, err := generateAudio(cfg, "HGfmRCkOadeBGOFiuKZW", aiSmartResp.FullScript)
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

				timestamps, err := cfg.getCutTimestamps(audioFile, aiSmartResp)
				if err != nil {
					respondWithError(w, http.StatusInternalServerError, "error getting timestamps", err.Error())
					return
				}
				//SPEECH TO TEXT DEEPGRAM

				key, err := getAssestPath("audio/mp3")
				if err != nil {
					respondWithError(w, http.StatusInternalServerError, "error getting path", err.Error())
					return
				}
				key = filepath.Join(user.ID, client.ID, "series", key)

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
				series.S3Url = sql.NullString{String: url}
				err = cfg.db.UpdateVideoSeries(r.Context(), database.UpdateVideoSeriesParams{
					Title:       series.Title,
					Description: series.Description,
					S3Url:       series.S3Url,
					ClientID:    series.ClientID,
					ID:          series.ID,
					UserID:      series.UserID,
				})
				if err != nil {
					respondWithError(w, http.StatusInternalServerError, "error updating video series", err.Error())
					return
				}

				series, err = cfg.dbVideoSeriesToSignedVideoSeries(series)
				if err != nil {
					respondWithError(w, http.StatusInternalServerError, "error generating presigned url", err.Error())
					return
				}

				log.Println(series.S3Url.String)

			log.Println("creating avatar... \n\n")
			signedVid, err := cfg.dbVideoToSignedVideo(video)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "error gettign signed video", err.Error())
				return
			}

			avatarId := ""
			if user.AvatarID == "unset" {
				avatarId, err := cfg.generateAvatar(user, signedVid.S3Url.String)
				if err != nil {
					respondWithError(w, http.StatusInternalServerError, "error generating an avatar", err.Error())
					return
				}
				_, err = cfg.db.UpdateUserAvatarId(r.Context(), database.UpdateUserAvatarIdParams{
					AvatarID: avatarId,
					ID:       user.ID,
				})

				if err != nil {
					respondWithError(w, http.StatusInternalServerError, "err updating avatar id", err.Error())
					return
				}

				log.Println("successfully created new avatar id")

			} else {
				avatarId = user.AvatarID
				log.Println("got avatar id")
			}

			log.Println("generating video... \n\n")

			signedSeries, err := cfg.dbVideoSeriesToSignedVideoSeries(series)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "error getting signed series", err.Error())
				return
			}

			videoId, err := cfg.generateVideoAndGetId(aiSmartResp.FullScript, avatarId)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "error generating video", err.Error())
				return
			}

		log.Println("downloading video... \n\n")
		videoPath, err := cfg.downloadVideo("39b1811887", user.ID+".mp4")
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "error downloading file", err.Error())
			return
		}

	*/

	videoPath := "temp/video-447281690.mp4"
	log.Println("video downloaded")
	audioPath, err := extractAudio(videoPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error extracting audio", err.Error())
		return
	}
	aiSmartResp := openaiSmartResponse{
		FullScript: "Hi Andrew I’m Murph, your local real estate expert. I’m so excited to connect with you about your home at —thanks for reaching out!  I’ve been helping families sell their homes for over a decade. I started small, just like many of my clients, and I’m proud to work with a tight-knit team at South Morang. We’re all about making the selling process simple and stress-free for you, using honest, tried-and-true strategies to get the best results.  I understand you have a wonderful dog named Frankie. I’m sure he loves having such a lovely space to play and relax.  I’d love to offer you a free consultation to see how much your home is worth and help you plan your next steps. I’ll reach out soon to set up a chat!  I can’t wait to work with you—talk soon!",
		CutWords: []CutWord{
			{Word: "out!", Index: 23},
			{Word: "results.", Index: 76},
		},
	}
	timestamps, err := cfg.getCutTimestamps(audioPath, aiSmartResp)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting timestamps", err.Error())
		return
	}

	log.Println("TIMESTAMPS: ", timestamps[0], ", ", timestamps[1])
	outputFile, err := cfg.editAndUpload(videoPath, "test.mp4", user.ID, timestamps)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error editing the video", err.Error())
		return
	}

	log.Println("Successfully edited: ", outputFile)

	respondWithJSON(w, http.StatusOK, "ok")

}

func downloadVideoTemplate(s3url string) (string, error) {
	resp, err := http.Get(s3url)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	filename := fmt.Sprintf("video_%s.mp4", uuid.New().String())
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}

	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return filename, err
}

func combineAudioVideo(videoFile, audioFile, outputFile string) error {
	cmd := exec.Command(
		"ffmpeg",
		"-i", videoFile,
		"-i", audioFile,
		"-c:v", "copy",
		"-c:a", "aac", // or "copy" if you want to keep MP3
		"-map", "0:v:0",
		"-map", "1:a:0",
		"-shortest",
		outputFile,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error combining audio and video: %w, output: %s", err, output)
	}
	return nil
}

func (cfg *apiConfig) dbVideoSeriesToSignedVideoSeries(series database.VideoSeries) (database.VideoSeries, error) {
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
	series.S3Url = sql.NullString{String: presigned, Valid: presigned != ""}
	return series, nil

}

func (cfg *apiConfig) handlerGenerateVideo(w http.ResponseWriter, r *http.Request) {
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

	client, err := cfg.db.GetClientById(r.Context(), params.ClientID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting client", err.Error())
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

	aiResp, err := getResponse(cfg.openaiClient, thread.ThreadID, runID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting ai response", err.Error())
		return
	}
	log.Println("Ai response got: ", aiResp)

	audioBytes, err := generateAudio(cfg, "HGfmRCkOadeBGOFiuKZW", aiResp.FullScript)
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

	vidTemplate, err := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error signing video", err.Error())
		return
	}
	videoFile, err := downloadVideoTemplate(vidTemplate.S3Url.String)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error saving video file", err.Error())
		return
	}
	log.Println("video file successfully downloaded\n")
	defer os.Remove(videoFile)

	outputFile := fmt.Sprintf("output_%s.mp4", uuid.New().String())
	err = combineAudioVideo(videoFile, audioFile, outputFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error combinging audio and video", err.Error())
		return
	}
	defer os.Remove(outputFile)
	log.Println("output file successfully made\n")

	key, _ := getAssestPath("media/mp4")
	key = filepath.Join(user.ID, client.ID, "series", key)

	processedFilePath, err := processVideoForFastStart(outputFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error processing video", err.Error())
		return
	}
	defer os.Remove(processedFilePath)

	processedFile, err := os.Open(processedFilePath)
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
		ContentType: aws.String("video/mp4"),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading file to s3", err.Error())
		return
	}

	url := fmt.Sprintf("%s,%s", cfg.s3Bucket, key)
	series.S3Url = sql.NullString{String: url}
	err = cfg.db.UpdateVideoSeries(r.Context(), database.UpdateVideoSeriesParams{
		Title:       series.Title,
		Description: series.Description,
		S3Url:       series.S3Url,
		ClientID:    series.ClientID,
		ID:          series.ID,
		UserID:      series.UserID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video series", err.Error())
		return
	}

	series, err = cfg.dbVideoSeriesToSignedVideoSeries(series)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error generating presigned url", err.Error())
		return
	}

	log.Println(series.S3Url.String)

	respondWithJSON(w, http.StatusOK, series)

}
