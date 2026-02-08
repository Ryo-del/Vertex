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
	pay "Vertex/internal/pay"
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

func premiumMiddleware(repo repo.Repository) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userIDVal := r.Context().Value("userID")
			userID, ok := userIDVal.(int)
			if !ok || userID == 0 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			prof, err := repo.GetProfileByID(r.Context(), userID)
			if err != nil {
				http.Error(w, "DB error", http.StatusInternalServerError)
				return
			}
			if prof.PremiumUntil != nil {
				if time.Now().After(*prof.PremiumUntil) {
					_ = repo.ClearPremium(r.Context(), userID)
					http.Error(w, "Premium required", http.StatusPaymentRequired)
					return
				}
			} else if !prof.IsPremium {
				http.Error(w, "Premium required", http.StatusPaymentRequired)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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
	payClient := pay.NewClient(os.Getenv("TINKOFF_TERMINAL_KEY"), os.Getenv("TINKOFF_PASSWORD"))
	publicURL := os.Getenv("PUBLIC_URL")
	if publicURL == "" {
		publicURL = "https://localhost"
	}

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

	secureApi.HandleFunc("/premium/init", func(w http.ResponseWriter, r *http.Request) {
		userIDVal := r.Context().Value("userID")
		userID, ok := userIDVal.(int)
		if !ok || userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if payClient.TerminalKey == "" || payClient.Password == "" {
			http.Error(w, "Payment not configured", http.StatusServiceUnavailable)
			return
		}
		orderID := fmt.Sprintf("premium-%d-%d", userID, time.Now().Unix())
		req := pay.InitRequest{
			Amount:          29900,
			OrderID:         orderID,
			Description:     "Premium subscription (1 month)",
			SuccessURL:      publicURL + "/payment/success",
			FailURL:         publicURL + "/payment/fail",
			NotificationURL: publicURL + "/api/premium/notify",
			CustomerKey:     fmt.Sprintf("user-%d", userID),
		}
		resp, err := payClient.Init(req)
		if err != nil {
			http.Error(w, "Payment init error", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"payment_url": resp.PaymentURL,
			"payment_id":  resp.PaymentID,
			"order_id":    orderID,
		})
	}).Methods("POST")

	api.HandleFunc("/premium/notify", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		token, _ := payload["Token"].(string)
		if !payClient.VerifyToken(payload, token) {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		status, _ := payload["Status"].(string)
		if status == "CONFIRMED" || status == "AUTHORIZED" {
			// Activation will be confirmed on success callback for local testing
		}
		w.WriteHeader(http.StatusOK)
	}).Methods("POST")

	secureApi.HandleFunc("/premium/status", func(w http.ResponseWriter, r *http.Request) {
		userIDVal := r.Context().Value("userID")
		userID, ok := userIDVal.(int)
		if !ok || userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		prof, err := userRepo.GetProfileByID(r.Context(), userID)
		if err != nil {
			http.Error(w, "DB error", http.StatusInternalServerError)
			return
		}
		active := prof.IsPremium
		if prof.PremiumUntil != nil {
			active = time.Now().Before(*prof.PremiumUntil)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"active":        active,
			"premium_until": prof.PremiumUntil,
		})
	}).Methods("GET")

	secureApi.HandleFunc("/premium/request", func(w http.ResponseWriter, r *http.Request) {
		userIDVal := r.Context().Value("userID")
		userID, ok := userIDVal.(int)
		if !ok || userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		prof, err := userRepo.GetProfileByID(r.Context(), userID)
		if err != nil {
			http.Error(w, "DB error", http.StatusInternalServerError)
			return
		}
		line := fmt.Sprintf("%s | user_id=%d | login=%s | email=%s\n", time.Now().Format(time.RFC3339), userID, prof.Login, prof.Email)
		f, err := os.OpenFile("./admin/tikets/premium_requests.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = f.WriteString(line)
			_ = f.Close()
		}
		w.WriteHeader(http.StatusOK)
	}).Methods("POST")

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

	premiumApi := secureApi.PathPrefix("/tools-sp").Subrouter()
	premiumApi.Use(premiumMiddleware(userRepo))
	premiumApi.HandleFunc("/piles/calc", pilesSpH.Calc).Methods("POST")
	premiumApi.HandleFunc("/beam/calc", beamSpH.Calc).Methods("POST")
	premiumApi.HandleFunc("/loads/calc", loadsSpH.Calc).Methods("POST")
	premiumApi.HandleFunc("/anchors/calc", anchorsSpH.Calc).Methods("POST")
	premiumApi.HandleFunc("/joints/calc", jointsSpH.Calc).Methods("POST")
	premiumApi.HandleFunc("/deflection/calc", deflectionSpH.Calc).Methods("POST")
	premiumApi.HandleFunc("/column/calc", columnSpH.Calc).Methods("POST")
	premiumApi.HandleFunc("/slab/calc", slabSpH.Calc).Methods("POST")
	premiumApi.HandleFunc("/report/pdf", reportSpH.Generate).Methods("POST")

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
	mux.PathPrefix("/pay/").
		Handler(http.StripPrefix("/pay/", http.FileServer(http.Dir("./static/pay/"))))
	authFileServer := http.FileServer(http.Dir("./static/auth"))
	mux.PathPrefix("/auth/").
		Handler(authEnv.RedirectIfLoggedIn(http.StripPrefix("/auth", authFileServer)))
	profileFileServer := http.FileServer(http.Dir("./static/profile"))
	mux.Handle("/profile/{id:[0-9]+}", authEnv.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/profile/index.html")
	})))
	mux.Handle("/payment/success", authEnv.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paymentID := r.URL.Query().Get("PaymentId")
		userIDVal := r.Context().Value("userID")
		userID, ok := userIDVal.(int)
		if !ok || userID == 0 {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if payClient.TerminalKey == "" || payClient.Password == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if paymentID != "" {
			if st, err := payClient.GetState(paymentID); err == nil {
				if st.Status == "CONFIRMED" || st.Status == "AUTHORIZED" {
					until := time.Now().Add(30 * 24 * time.Hour)
					_ = userRepo.SetPremiumUntil(r.Context(), userID, until)
				}
			}
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})))
	mux.Handle("/payment/fail", authEnv.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})))
	mux.PathPrefix("/profile/").
		Handler(authEnv.AuthMiddleware(http.StripPrefix("/profile", profileFileServer)))
	mux.PathPrefix("/docs/").
		Handler(authEnv.AuthMiddleware(http.StripPrefix("/docs", http.FileServer(http.Dir("./docs")))))

	mux.PathPrefix("/qr/").
		Handler(http.StripPrefix("/qr", http.FileServer(http.Dir("./QR"))))
	payFileServer := http.FileServer(http.Dir("./static/pay"))
	mux.PathPrefix("/pay/").
		Handler(authEnv.AuthMiddleware(http.StripPrefix("/pay", payFileServer)))
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
