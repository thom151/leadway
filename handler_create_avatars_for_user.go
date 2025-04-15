package main

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/database"
)

func (cfg *apiConfig) handlerCreateAvatarForUser(w http.ResponseWriter, r *http.Request) {
	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	type parameters struct {
		TemplateType string `json:"template_type"`
		AvatarID     string `json:"avatar_id"`
		Title        string `json:"title"`
		Description  string `json:"description"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't decode params", err.Error())
		return
	}
	avatarUUID := uuid.New().String()
	avatars, err := cfg.db.CreateAvatar(r.Context(), database.CreateAvatarParams{
		ID:           avatarUUID,
		TemplateType: params.TemplateType,
		Title:        params.Title,
		Description:  sql.NullString{String: params.Description, Valid: params.Description != ""},
		UserID:       user.ID,
		AvatarID:     params.AvatarID,
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating avatar", err.Error())
		return
	}

	userWithAvatarGroup, err := cfg.db.UpdateUserAvatarId(r.Context(), database.UpdateUserAvatarIdParams{
		AvatarID: avatars.AvatarID,
		ID:       user.ID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating avatar group", err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, userWithAvatarGroup)
}
