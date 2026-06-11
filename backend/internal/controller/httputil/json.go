package httputil

import (
	"encoding/json"
	"net/http"
)

const contentTypeJSON = "application/json"

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
