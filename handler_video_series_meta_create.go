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
		Title         string `json:"title"`
		Personalized  string `json:"personalize"`
		ClientName    string `json:"client_name"`
		ClientAddress string `json:"client_address"`
		UserID        string `json:"user_id"`
		TemplateID    string `json:"template_id"`
	}

	decoder := json.NewDecoder(r.Body)
	var parameters params
	err = decoder.Decode(&parameters)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error decoding video series meta parameters", err.Error())
		return
	}

	template, err := cfg.db.GetVideoById(r.Context(), parameters.TemplateID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting video template", err.Error())
		return
	}

	log.Println("Personalized: ", parameters.Personalized)

	videoSeriesMeta, err := cfg.db.CreateVideoSeriesMeta(r.Context(), database.CreateVideoSeriesMetaParams{
		ID:          uuid.New().String(),
		UserID:      user.ID,
		Title:       parameters.Title,
		Description: sql.NullString{String: parameters.Personalized, Valid: parameters.Personalized != ""},
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating video series meta", err.Error())
		return
	}

	type response struct {
		VideoSeriesMeta database.VideoSeries
		TemplateID      string
		ClientName      string
		ClientAddress   string
	}

	resp := response{
		VideoSeriesMeta: videoSeriesMeta,
		TemplateID:      template.ID,
		ClientName:      parameters.ClientName,
		ClientAddress:   parameters.ClientAddress,
	}

	respondWithJSON(w, http.StatusOK, resp)
}
