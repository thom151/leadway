package main

import "net/http"

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		if _, err := w.Write([]byte("Reset is only allowed in dev environment.")); err != nil {
			respondWithError(w, http.StatusInternalServerError, "error writing reset in dev env", err.Error())
			return
		}
		return
	}

	cfg.fileserverHits.Store(0)
	if err := cfg.db.Reset(r.Context()); err != nil {
		respondWithError(w, http.StatusInternalServerError, "error resetting db", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("Hits reset to 0 and database reset to initial state."))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error writing reset", err.Error())
		return
	}
}
