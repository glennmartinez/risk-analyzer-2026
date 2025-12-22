package handlers

import (
	"fmt"
	"net/http"
)

// HomeHandler godoc
// @Summary Home page
// @Description Returns a welcome message for the API server
// @Tags general
// @Accept json
// @Produce text/plain
// @Success 200 {string} string "Welcome to the Grok Server!"
// @Router / [get]
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	fmt.Fprintln(w, "Welcome to the Grok Server!")
}
