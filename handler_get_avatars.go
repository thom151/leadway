package main

import (
	"encoding/json"
	"net/http"
)

type hey_avatars struct {
	Error any `json:"error"`
	Data  struct {
		AvatarList []struct {
			AvatarID        string `json:"avatar_id"`
			AvatarName      string `json:"avatar_name"`
			Gender          string `json:"gender"`
			PreviewImageURL string `json:"preview_image_url"`
			PreviewVideoURL string `json:"preview_video_url"`
			Premium         bool   `json:"premium"`
			Type            any    `json:"type"`
			Tags            []any  `json:"tags"`
		} `json:"avatar_list"`
	} `json:"data"`
}

func (cfg *apiConfig) handlerGetAvatars(w http.ResponseWriter, r *http.Request) {
	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unauthorized: ", err.Error())
		return
	}

	avatarGroup, err := cfg.db.GetAvatarsByUser(r.Context(), user.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting avatar", err.Error())
		return
	}

	url := "https://api.heygen.com/v2/avatar_group/" + avatarGroup.AvatarID + "/avatars"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error making new request", err.Error())
		return
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("x-api-key", cfg.heygenApiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		respondWithError(w, http.StatusInsufficientStorage, "error executing request", err.Error())
		return
	}

	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	var avatars hey_avatars
	err = decoder.Decode(&avatars)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error decoding avatar parameters", err.Error())
		return
	}

	renderTemplate(w, "avatars", avatars)

}
