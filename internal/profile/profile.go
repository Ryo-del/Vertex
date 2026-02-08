package profile

import (
	"Vertex/internal/repo"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type ProfileHandler struct {
	Repo repo.Repository
}

type Registerrequest struct {
	Login       string `json:"login"`
	Email       string `json:"email"`
	Description string `json:"description"`
	Avatar_url  string `json:"avatar_url"`
}

type UpdateProfileRequest struct {
	Login       string `json:"login"`
	Description string `json:"description"`
}

const MaxUploadSize = 10 << 20 // 10MB
func (h *ProfileHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	idVal := r.Context().Value("userID")
	userID, ok := idVal.(int)
	if !ok || userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		http.Error(w, "File too big", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll("./static/uploads", 0755); err != nil {
		http.Error(w, "Storage error", http.StatusInternalServerError)
		return
	}

	file, handler, err := r.FormFile("photo")
	if err != nil {
		http.Error(w, "Invalid file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileName := fmt.Sprintf("%d%s", time.Now().UnixNano(), filepath.Ext(handler.Filename))
	imagePath := "/uploads/" + fileName
	fullPath := "./static" + imagePath

	f, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		http.Error(w, "Storage error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		http.Error(w, "Storage error", http.StatusInternalServerError)
		return
	}

	// Обновляем БД используя проверенный login из токена
	if err := h.Repo.UpdateAvatar(r.Context(), userID, imagePath); err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if idStr, ok := vars["id"]; ok && idStr != "" {
		targetID, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Некорректный id", http.StatusBadRequest)
			return
		}
		prof, err := h.Repo.GetProfileByID(r.Context(), targetID)
		if err != nil {
			http.Error(w, "Профиль не найден", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(prof)
		return
	}

	idVal := r.Context().Value("userID")
	userID, ok := idVal.(int)
	if !ok || userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	prof, err := h.Repo.GetProfileByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Профиль не найден", http.StatusNotFound)
		return
	}

	if prof.PremiumUntil != nil && time.Now().After(*prof.PremiumUntil) {
		_ = h.Repo.ClearPremium(r.Context(), userID)
		prof.IsPremium = false
		prof.PremiumUntil = nil
	}

	json.NewEncoder(w).Encode(prof)
}

func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	idVal := r.Context().Value("userID")
	userID, ok := idVal.(int)
	if !ok || userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if _, err := h.Repo.UpdateProfile(r.Context(), userID, req.Login, req.Description); err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
