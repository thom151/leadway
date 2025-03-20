package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/thom151/leadme/internal/database"
)

func (cfg *apiConfig) handlerCreateClient(w http.ResponseWriter, r *http.Request) {

	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid user", err.Error())
		return
	}
	type parameters struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		Address string `json:"address"`
	}

	decoder := json.NewDecoder(r.Body)
	var params parameters
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error decoding client parameters", err.Error())
		return
	}

	client, err := cfg.db.CreateClients(r.Context(), database.CreateClientsParams{
		ID:      uuid.New().String(),
		AgentID: user.ID,
		Name:    params.Name,
		Email:   params.Email,
		Phone:   sql.NullString{String: params.Phone},
		Address: sql.NullString{String: params.Address},
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating client", err.Error())
	}

	log.Println("Client successfuly created")
	respondWithJSON(w, http.StatusOK, client)

}
