package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/auth"
	"github.com/thom151/leadme/internal/database"
)

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case http.MethodGet:
		http.ServeFile(w, r, "./templates/login.html")
		return
	case http.MethodPost:
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "couldn't parse form data", http.StatusBadRequest)
			return
		}
		type parameters struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		type response struct {
			User
			Token        string `json:"token"`
			RefreshToken string `json:"refresh_token"`
		}

		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		err = decoder.Decode(&params)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "couldn't decode params", err.Error())
			return
		}

		user, err := cfg.db.GetUserByEmail(r.Context(), params.Email)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "incorrect email/password", err.Error())
			return
		}

		err = auth.CheckPasswordHash(params.Password, user.PasswordHash)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "incorrect email or password", err.Error())
			return
		}

		userUUID, err := uuid.Parse(user.ID)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error parsing id in login", err.Error())
			return
		}
		accToken, err := auth.MakeJWT(userUUID, cfg.secret, time.Hour)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed creating acc token", err.Error())
			return
		}

		refreshToken, err := auth.MakeRefreshToken()
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed to created refresh token", err.Error())
			return
		}

		refreshParams := database.CreateRefreshTokenParams{
			Token:     refreshToken,
			UserID:    user.ID,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			ExpiresAt: time.Now().Add(60 * 24 * time.Hour).Format(time.RFC3339),
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}

		_, err = cfg.db.CreateRefreshToken(r.Context(), refreshParams)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed to store refresh token", err.Error())
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "acc_token",
			Value:    accToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			Expires:  time.Now().Add(24 * time.Hour),
		})

		//http.Redirect(w, r, "/app", http.StatusSeeOther)

		respondWithJSON(w, http.StatusOK, response{
			User: User{
				Email:     user.Email,
				ID:        user.ID,
				CreatedAt: user.CreatedAt,
				UpdatedAt: user.UpdatedAt,
			},
			Token:        accToken,
			RefreshToken: refreshToken,
		})

		return
	}

}
