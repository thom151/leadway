package main

import (
	"net/http"

	"github.com/thom151/leadme/internal/database"
)

func (cfg *apiConfig) handlerStartVideoSeries(w http.ResponseWriter, r *http.Request) {
	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid user", err.Error())
		return
	}

	videos, err := cfg.db.GetVideosByUser(r.Context(), user.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting videos", err.Error())
		return
	}

	for i, video := range videos {
		video, err = cfg.dbVideoToSignedVideo(video)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "error generating presigned url", err.Error())
			return
		}
		videos[i] = video
	}

	clients, err := cfg.db.GetAgentClients(r.Context(), user.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting clients", err.Error())
		return
	}

	type response struct {
		Videos  []database.VideoTemplate
		Clients []database.Client
	}

	resp := response{
		Videos:  videos,
		Clients: clients,
	}

	renderTemplate(w, "series", resp)
}
