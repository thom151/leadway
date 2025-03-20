package main

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/database"
)

func (cfg *apiConfig) handlerVideoMetaCreate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		renderTemplate(w, "video_upload", nil)
		return
	case http.MethodPost:

		user, err := validateAndReturnUser(r, cfg)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "invalid user", err.Error())
			return
		}
		type params struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			UserID      string `json:"user_id"`
		}

		decoder := json.NewDecoder(r.Body)
		var parameters params
		err = decoder.Decode(&parameters)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "error decoding parameters", err.Error())
			return
		}

		videoMeta, err := cfg.db.CreateVideoMeta(r.Context(), database.CreateVideoMetaParams{
			ID:          uuid.New().String(),
			UserID:      user.ID,
			Title:       parameters.Title,
			Description: sql.NullString{String: parameters.Description, Valid: parameters.Description != ""},
		})

		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "error creating video meta", err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, videoMeta)
	}
}
