package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"
	"github.com/thom151/leadme/internal/database"
)

func (cfg *apiConfig) handlerCloneVoice(w http.ResponseWriter, r *http.Request) {

	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}
	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error parsing form", err.Error())
		return
	}
	file, header, err := r.FormFile("voice")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting voice", err.Error())
		return
	}
	defer file.Close()

	voiceName := r.FormValue("voice_name")
	if voiceName == "" {
		respondWithError(w, http.StatusBadRequest, "missing name", err.Error())
		return
	}
	fmt.Println("sending request...")
	client := resty.New()
	resp, err := client.R().
		SetHeader("xi-api-key", cfg.elevenApiKey).
		SetFileReader("files", header.Filename, file).
		SetFormData(map[string]string{
			"name": voiceName,
		}).
		Post("https://api.elevenlabs.io/v1/voices/add")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error sending request", err.Error())
		return
	}

	type clonedVoice struct {
		VoiceId              string `json:"voice_id"`
		RequiresVerification bool   `json:"requires_verification"`
	}

	var cloned clonedVoice
	err = json.Unmarshal(resp.Body(), &cloned)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error decoding response", err.Error())
		return
	}

	voiceParams := database.CreateVoiceAssistantParams{
		Name:          voiceName,
		ClonedVoiceID: cloned.VoiceId,
		UserID:        user.ID,
	}

	voice_assistant, err := cfg.db.CreateVoiceAssistant(r.Context(), voiceParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error saving voice", err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, voice_assistant)
}
