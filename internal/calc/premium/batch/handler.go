package batch

import (
	"encoding/json"
	"net/http"
)

type Handler struct{}

func (h *Handler) Beam(w http.ResponseWriter, r *http.Request) {
	var input BeamBatchInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	res, err := CalculateBeam(input)
	if err != nil {
		http.Error(w, "Calculation error", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
