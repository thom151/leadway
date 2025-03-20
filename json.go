package main

import (
	"encoding/json"
	"net/http"
)

func respondWithError(w http.ResponseWriter, code int, msg, errMsg string) {
	type ErrorResponse struct {
		Error        string `json:"error"`
		ErrorMessage string `json:"error_msg"`
	}

	decodeError := ErrorResponse{
		Error:        msg,
		ErrorMessage: errMsg,
	}
	errDat, _ := json.Marshal(decodeError)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err := w.Write(errDat)
	if err != nil {
		return
	}
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	resp, err := json.Marshal(payload)
	if err != nil {
		respondWithError(w, 500, "Something went wrong", err.Error())
	}
	_, err = w.Write([]byte(resp))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing resp", err.Error())
		return
	}

}
