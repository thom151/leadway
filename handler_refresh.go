package main

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/auth"
	"github.com/thom151/leadme/internal/database"
)

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {

	type response struct {
		Token string `json:"token"`
	}

	refToken, err := auth.GetBearerToken(r.Header, r.Cookies())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Bearer missing", err.Error())
		return
	}

	token, err := cfg.db.GetToken(r.Context(), refToken)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Token doesn't exist", err.Error())
		return
	}

	expiresAtTime, err := time.Parse(time.RFC3339, token.ExpiresAt)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error parsing time", err.Error())
		return
	}
	if expiresAtTime.Before(time.Now()) {
		respondWithError(w, http.StatusBadRequest, "Token expired", err.Error())
		return
	}
	userUUID, err := uuid.Parse(token.UserID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error parsing id in refresh", err.Error())
		return
	}
	newJWT, err := auth.MakeJWT(userUUID, cfg.secret, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Creating jwt failed", err.Error())
		return
	}

	if token.RevokedAt.Valid {
		respondWithError(w, http.StatusBadRequest, "Token has been revoked", err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, response{
		Token: newJWT,
	})

}

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header, r.Cookies())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find token", err.Error())
		return
	}

	token, err := cfg.db.GetToken(r.Context(), refreshToken)
	if err != nil {
		respondWithError(w, 401, "Token does not exist", err.Error())
		return
	}

	revokeTokenParams := database.RevokeTokenParams{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Token:     token.Token,
	}

	err = cfg.db.RevokeToken(r.Context(), revokeTokenParams)
	if err != nil {
		respondWithError(w, 500, "Token not revoked", err.Error())
	}

	respondWithJSON(w, 204, nil)
}
