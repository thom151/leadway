package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (cfg *apiConfig) handlerGenerateVideoFromTemaplate(w http.ResponseWriter, r *http.Request) {

	templateId := "323446"
	url := "https://api.heygen.com/v2/template/" + templateId + "/generate"

	payload := strings.NewReader("{\"caption\":false,\"callback_id\":\"<callback_id>\",\"title\":\"Untitled Video\",\"dimension\":{\"width\":1280,\"height\":720},\"include_gif\":false,\"enable_sharing\":false}")

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting request", err.Error())
		return
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")
	req.Header.Add("x-api-key", "NmI4MTdmN2ViMjU3NDgwNDliY2VmNjdkNTg3YzE3OWQtMTc0MjIwODY0MA==")

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	fmt.Println(string(body))

}
