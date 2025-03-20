package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/thom151/leadme/internal/database"
)

type parameters struct {
	VoiceID   string `json:"voice_id"`
	ContactID string `json:"contact_id"`
}

type SpeakResponse struct {
	StreamURL string `json:"stream_url"`
}

func (cfg *apiConfig) handlerSpeak(w http.ResponseWriter, r *http.Request) {

	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting user", err.Error())
		return
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't decode parameters", err.Error())
		return
	}

	c := cfg.openaiClient
	thread, err := cfg.db.GetThread(r.Context(), database.GetThreadParams{
		UserID:    user.ID,
		ContactID: params.ContactID,
	})
	if err != nil {
		threadID, err := genThread(c, user.Username)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "error generating thread", err.Error())
			return
		}

		thread, err = cfg.db.CreateThread(r.Context(), database.CreateThreadParams{
			UserID:    user.ID,
			ContactID: params.ContactID,
			ThreadID:  threadID.ID,
		})

		fmt.Println("thread generated")
	}

	err = sendMessage(c, thread.ThreadID, "yeah i'm interested")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error sending message", err.Error())
		return
	}

	runID, err := getRunID(c, thread.ThreadID, cfg.assistantID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting run id", err.Error())
		return
	}

	cloneResponse, err := getResponse(c, thread.ThreadID, runID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting clone response", err.Error())
		return
	}
	url := "https://api.elevenlabs.io/v1/text-to-speech/" + params.VoiceID + "/stream?output_format=mp3_44100_128"
	payload := strings.NewReader(fmt.Sprintf(`{"text": "%s", "model_id":"eleven_multilingual_v2"}`, cloneResponse))
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error making request", err.Error())
		return
	}
	req.Header.Add("xi-api-key", cfg.elevenApiKey)
	req.Header.Add("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't send request", err.Error())
		return
	}
	defer res.Body.Close()

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Transfer-Encoding", "chunked")
	io.Copy(w, res.Body)

}
