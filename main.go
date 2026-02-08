package main

import (
	auth "Vertex/internal/auth"
	anchorssp "Vertex/internal/calc/SP/anchors-SP"
	beamsp "Vertex/internal/calc/SP/beam-SP"
	columnsp "Vertex/internal/calc/SP/column-SP"
	deflectionsp "Vertex/internal/calc/SP/deflection-SP"
	jointssp "Vertex/internal/calc/SP/joints-SP"
	loadssp "Vertex/internal/calc/SP/loads-SP"
	pilessp "Vertex/internal/calc/SP/piles-SP"
	reportsp "Vertex/internal/calc/SP/report-SP"
	slabsp "Vertex/internal/calc/SP/slab-SP"
	anchors "Vertex/internal/calc/anchors"
	beam "Vertex/internal/calc/beam"
	column "Vertex/internal/calc/column"
	deflection "Vertex/internal/calc/deflection"
	joints "Vertex/internal/calc/joints"
	loads "Vertex/internal/calc/loads"
	piles "Vertex/internal/calc/piles"
	report "Vertex/internal/calc/report"
	slab "Vertex/internal/calc/slab"
	profile "Vertex/internal/profile"
	repo "Vertex/internal/repo"
	"context"
	"database/sql"

	"encoding/json"
	"fmt"
	"io/fs"
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
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
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

	api := mux.PathPrefix("/api").Subrouter()
	api.Use(limiter.LimitMiddleware)

	api.HandleFunc("/login", authEnv.AuthHandler).Methods("POST")
	api.HandleFunc("/register", authEnv.RegisterHandler).Methods("POST")

	secureApi := api.PathPrefix("/user").Subrouter()
	secureApi.Use(authEnv.AuthMiddleware)

	secureApi.HandleFunc("/profile", profileH.GetProfile).Methods("GET")
	secureApi.HandleFunc("/profile", profileH.UpdateProfile).Methods("PATCH", "PUT")
	secureApi.HandleFunc("/profile/{id:[0-9]+}", profileH.GetProfile).Methods("GET")
	secureApi.HandleFunc("/upload-avatar", profileH.UploadAvatar).Methods("POST")

	pilesH := &piles.Handler{}
	secureApi.HandleFunc("/tools/piles/calc", pilesH.Calc).Methods("POST")

	beamH := &beam.Handler{}
	anchorsH := &anchors.Handler{}
	columnH := &column.Handler{}
	deflectionH := &deflection.Handler{}
	jointsH := &joints.Handler{}
	loadsH := &loads.Handler{}
	reportH := &report.Handler{}
	slabH := &slab.Handler{}
	beamSpH := &beamsp.Handler{}
	anchorsSpH := &anchorssp.Handler{}
	columnSpH := &columnsp.Handler{}
	deflectionSpH := &deflectionsp.Handler{}
	jointsSpH := &jointssp.Handler{}
	loadsSpH := &loadssp.Handler{}
	pilesSpH := &pilessp.Handler{}
	reportSpH := &reportsp.Handler{}
	slabSpH := &slabsp.Handler{}

	secureApi.HandleFunc("/tools/beam/calc", beamH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools/loads/calc", loadsH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools/anchors/calc", anchorsH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools/deflection/calc", deflectionH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools/joints/calc", jointsH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools/report/pdf", reportH.Generate).Methods("POST")
	secureApi.HandleFunc("/tools/column/calc", columnH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools/slab/calc", slabH.Calc).Methods("POST")

	secureApi.HandleFunc("/tools-sp/piles/calc", pilesSpH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools-sp/beam/calc", beamSpH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools-sp/loads/calc", loadsSpH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools-sp/anchors/calc", anchorsSpH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools-sp/joints/calc", jointsSpH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools-sp/deflection/calc", deflectionSpH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools-sp/column/calc", columnSpH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools-sp/slab/calc", slabSpH.Calc).Methods("POST")
	secureApi.HandleFunc("/tools-sp/report/pdf", reportSpH.Generate).Methods("POST")

	secureApi.HandleFunc("/docs/list", func(w http.ResponseWriter, r *http.Request) {
		type Doc struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		var docs []Doc
		fs.WalkDir(os.DirFS("./docs"), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			docs = append(docs, Doc{Name: d.Name(), Path: path})
			return nil
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(docs)
	}).Methods("GET")

	mux.PathPrefix("/uploads/").
		Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir("./static/uploads/"))))

	authFileServer := http.FileServer(http.Dir("./static/auth"))
	mux.PathPrefix("/auth/").
		Handler(authEnv.RedirectIfLoggedIn(http.StripPrefix("/auth", authFileServer)))
	profileFileServer := http.FileServer(http.Dir("./static/profile"))
	mux.Handle("/profile/{id:[0-9]+}", authEnv.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/profile/index.html")
	})))
	mux.PathPrefix("/profile/").
		Handler(authEnv.AuthMiddleware(http.StripPrefix("/profile", profileFileServer)))
	mux.PathPrefix("/docs/").
		Handler(authEnv.AuthMiddleware(http.StripPrefix("/docs", http.FileServer(http.Dir("./docs")))))
	mainFileServer := http.FileServer(http.Dir("./static/main"))
	mux.PathPrefix("/").
		Handler(mainFileServer)

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
