package main

import (
	"net/http"

	"github.com/thom151/leadme/internal/auth"
	"github.com/thom151/leadme/internal/database"
)

func validateAndReturnUser(r *http.Request, cfg *apiConfig) (database.User, error) {
	token, err := auth.GetBearerToken(r.Header, r.Cookies())
	if err != nil {
		return database.User{}, err
	}

	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		return database.User{}, err
	}

	user, err := cfg.db.GetUserByID(r.Context(), userID.String())
	if err != nil {
		return database.User{}, err
	}

	return user, nil

}
