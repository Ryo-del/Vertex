package autodesign

import (
	"encoding/json"
	"net/http"
)

type Handler struct{}

func (h *Handler) Beam(w http.ResponseWriter, r *http.Request) {
	var input BeamAutoInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	res, err := Beam(input)
	if err != nil {
		http.Error(w, "Calculation error", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
