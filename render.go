package main

import (
	"net/http"
	"text/template"
)

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	tmpls := []string{"templates/layout.html", "templates/" + tmpl + ".html"}
	t, err := template.ParseFiles(tmpls...)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error parsing templates", err.Error())
		return
	}

	err = t.ExecuteTemplate(w, "layout", data)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error executing templates", err.Error())
		return
	}
}
