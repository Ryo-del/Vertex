package main

import (
	auth "Vertex/internal/auth"
	profile "Vertex/internal/profile"
	repo "Vertex/internal/repo"
	"context"
	"database/sql"

	"fmt"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

var wg sync.WaitGroup

func CORS(mux *mux.Router) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") //у меня нет домена это тестовый сервер
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func HandleList(mux *mux.Router, db *sql.DB) {
	userRepo := repo.NewPostgresUserDB(db)
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// Load TOKEN_KEY from environment
	tokenKey := os.Getenv("TOKEN_KEY")
	if tokenKey == "" {
		log.Fatal("TOKEN_KEY environment variable is not set")
	}

	authEnv := &auth.Authenv{JWTkey: []byte(tokenKey), Repo: userRepo}
	profileH := &profile.ProfileHandler{Repo: userRepo}

	limiter := auth.NewIPRateLimiter(1, 3)

	// --- 1. API СЛОЙ (ВЫСШИЙ ПРИОРИТЕТ) ---
	api := mux.PathPrefix("/api").Subrouter()
	api.Use(limiter.LimitMiddleware)

	// Публичные эндпоинты (Доступны всем без токена)
	api.HandleFunc("/login", authEnv.AuthHandler).Methods("POST")
	api.HandleFunc("/register", authEnv.RegisterHandler).Methods("POST")
	secureApi := api.PathPrefix("/user").Subrouter()
	secureApi.Use(authEnv.AuthMiddleware) // Твой Middleware для проверки токена
	secureApi.HandleFunc("/profile", profileH.GetProfile).Methods("GET")
	secureApi.HandleFunc("/upload-avatar", profileH.UploadAvatar).Methods("POST")
	// --- 2. МЕДИА-КОНТЕНТ ---
	// Картинки должны отдаваться без всяких проверок авторизации
	uploadsStatic := http.StripPrefix("/uploads/", http.FileServer(http.Dir("./static/uploads/")))
	mux.PathPrefix("/uploads/").Handler(uploadsStatic)

	// --- 3. СТАТИЧЕСКИЕ СТРАНИЦЫ (ИНТЕРФЕЙС) ---

	// Авторизация (вход/рег)
	authFileServer := http.FileServer(http.Dir("./static/auth/"))
	mux.PathPrefix("/auth/").Handler(authEnv.RedirectIfLoggedIn(http.StripPrefix("/auth", authFileServer)))

	// Профиль (требует логин)
	profileFileServer := http.FileServer(http.Dir("./static/profile/"))
	mux.PathPrefix("/profile/").Handler(authEnv.AuthMiddleware(http.StripPrefix("/profile", profileFileServer)))

	// ГЛАВНАЯ СТРАНИЦА (НИЗШИЙ ПРИОРИТЕТ)
	// Она должна быть ПОСЛЕДНЕЙ и ПУБЛИЧНОЙ
	mainFileServer := http.FileServer(http.Dir("./static/main/"))
	mux.PathPrefix("/").Handler(authEnv.AuthMiddleware(mainFileServer))
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	db := auth.InitDB()
	defer db.Close()
	mux := mux.NewRouter()
	log.Println("Starting server on :443")
	HandleList(mux, db)
	handler := CORS(mux)

	server := &http.Server{
		Addr:    ":443",
		Handler: handler,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.ListenAndServeTLS("server.crt", "server.key"); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	fmt.Println("Shutdown signal received!")
	fmt.Println("Закрытие активных соединений")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Ошибка при остановке сервера: %v", err)
	}
	log.Println("Сервер успешно остановлен")

	wg.Wait()
}
