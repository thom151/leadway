package main

import "net/http"

func (cfg *apiConfig) handlerRecordVideo(w http.ResponseWriter, r *http.Request) {
	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "access unauthorized", err.Error())
		return
	}

	renderTemplate(w, "record", user)
}
