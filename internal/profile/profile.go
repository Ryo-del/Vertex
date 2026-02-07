package profile

import (
	"Vertex/internal/repo"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
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

const MaxUploadSize = 10 << 20 // 10MB
func (h *ProfileHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	// 🛡 Достаем логин из контекста, а не из URL!
	val := r.Context().Value("userLogin")
	login, ok := val.(string)
	if !ok || login == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		http.Error(w, "File too big", http.StatusBadRequest)
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

	f, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		http.Error(w, "Storage error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	io.Copy(f, file)

	// Обновляем БД используя проверенный login из токена
	if err := h.Repo.UpdateAvatar(r.Context(), login, imagePath); err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	// 🛡 Безопасное извлечение из контекста
	val := r.Context().Value("userLogin")
	login, ok := val.(string)

	if !ok || login == "" {
		log.Println("[Auth Error] userLogin not found in context or not a string")
		http.Error(w, "Unauthorized: identity missing", http.StatusUnauthorized)
		return
	}

	// Теперь работаем с login безопасно
	prof, err := h.Repo.GetProfileByLogin(r.Context(), login)
	if err != nil {
		log.Printf("[DB Error] Profile not found for %s: %v", login, err)
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prof)
}
