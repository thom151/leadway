package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/auth"
	"github.com/thom151/leadme/internal/database"
)

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	PasswordHash string `json:"password"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func (cfg *apiConfig) handlerUsersCreate(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case http.MethodGet:
		http.ServeFile(w, r, "./templates/signup.html")
		return
	case http.MethodPost:
		type parameters struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		type response struct {
			User
		}

		decoder := json.NewDecoder(r.Body)
		reqParams := parameters{}
		err := decoder.Decode(&reqParams)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "error decording parameters", err.Error())
			return
		}

		hashed, err := auth.HashPassword(reqParams.Password)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "error hashing password", err.Error())
			return
		}

		userParams := database.CreateUserParams{
			Email:        reqParams.Email,
			PasswordHash: hashed,
			Username:     reqParams.Username,
			ID:           uuid.New().String(),
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		}

		user, err := cfg.db.CreateUser(r.Context(), userParams)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Couldn't create user", err.Error())
			return
		}

		log.Println(user.Email + "created successfully, redirecting to /app")

		//http.Redirect(w, r, "/app", http.StatusFound)

		respondWithJSON(w, http.StatusOK, response{
			User: User{
				ID:           user.ID,
				PasswordHash: user.PasswordHash,
				Username:     user.Username,
				Email:        user.Email,
				CreatedAt:    user.CreatedAt,
				UpdatedAt:    user.UpdatedAt,
			},
		})

		return
	}

}
