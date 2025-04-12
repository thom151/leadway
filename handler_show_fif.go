package main

import "net/http"

func (cfg *apiConfig) handlerShowFif(w http.ResponseWriter, r *http.Request) {
	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "unauthorized access", err.Error())
		return
	}

	seriesID := r.PathValue("seriesID")
	if seriesID == "" {
		respondWithError(w, http.StatusBadRequest, "missing series id", err.Error())
		return
	}

	series, err := cfg.db.GetVideoSeriesById(r.Context(), seriesID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting series", err.Error())
		return
	}

	if series.UserID != user.ID {
		respondWithError(w, http.StatusUnauthorized, "mismatch user and series users", "")
		return
	}

	signedSeries, err := cfg.dbFIFToSignedFIF(series)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting signed fif", err.Error())
		return
	}

	if !signedSeries.S3Url.Valid {
		respondWithError(w, http.StatusInternalServerError, "missing s3 url", err.Error())
		return
	}

	renderTemplate(w, "fif", signedSeries)
}
