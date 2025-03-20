package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/database"
)

func (cfg *apiConfig) handlerVideoSeriesMetaCreate(w http.ResponseWriter, r *http.Request) {
	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid user", err.Error())
		return
	}

	type params struct {
		Title        string `json:"title"`
		Personalized string `json:"personalize"`
		ClientID     string `json:"client_id"`
		UserID       string `json:"user_id"`
		VideoID      string `json:"video_id"`
	}

	decoder := json.NewDecoder(r.Body)
	var parameters params
	err = decoder.Decode(&parameters)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error decoding video series meta parameters", err.Error())
		return
	}

	client, err := cfg.db.GetClientById(r.Context(), parameters.ClientID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting client", err.Error())
		return
	}

	videoTemplate, err := cfg.db.GetVideoById(r.Context(), parameters.VideoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting video template", err.Error())
		return
	}

	log.Println("Personalized: ", parameters.Personalized)

	videoSeriesMeta, err := cfg.db.CreateVideoSeriesMeta(r.Context(), database.CreateVideoSeriesMetaParams{
		ID:          uuid.New().String(),
		UserID:      user.ID,
		ClientID:    parameters.ClientID,
		Title:       parameters.Title,
		Description: sql.NullString{String: parameters.Personalized, Valid: parameters.Personalized != ""},
	})

	log.Println("made: ", videoSeriesMeta.Description.String)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating video series meta", err.Error())
		return
	}

	type response struct {
		VideoSeriesMeta database.VideoSeries
		VideoTemplateID string
		ClientDetails   database.Client
	}

	resp := response{
		VideoSeriesMeta: videoSeriesMeta,
		VideoTemplateID: videoTemplate.ID,
		ClientDetails:   client,
	}

	respondWithJSON(w, http.StatusOK, resp)
}
