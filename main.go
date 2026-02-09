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
	pbatch "Vertex/internal/calc/premium/batch"
	pauto "Vertex/internal/calc/premium/autodesign"
	preco "Vertex/internal/calc/premium/recommend"
	pimport "Vertex/internal/calc/premium/importer"
	profile "Vertex/internal/profile"
	repo "Vertex/internal/repo"
	"context"
	"database/sql"

	"io"
	"mime/multipart"
	"encoding/json"
	"bytes"
	"fmt"
	"io/fs"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"strconv"

	"log"
	"net/http"
	"os"
	"strings"

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

func sendTelegramTicket(token string, peerID int64, text string, ticketID int) error {
	url := "https://api.telegram.org/bot" + token + "/sendMessage"
	payload := map[string]any{
		"chat_id": peerID,
		"text":    text,
		"reply_markup": map[string]any{
			"inline_keyboard": [][]map[string]string{
				{
					{"text": "✅ Подтвердить", "callback_data": fmt.Sprintf("approve:%d", ticketID)},
					{"text": "❌ Отклонить", "callback_data": fmt.Sprintf("reject:%d", ticketID)},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	_, err := http.Post(url, "application/json", bytes.NewReader(b))
	return err
}

type tgUpdate struct {
	UpdateID      int               `json:"update_id"`
	CallbackQuery *tgCallbackQuery  `json:"callback_query"`
	Message       *tgMessage        `json:"message"`
}

type tgCallbackQuery struct {
	ID      string     `json:"id"`
	Data    string     `json:"data"`
	Message *tgMessage `json:"message"`
}

type tgMessage struct {
	MessageID int    `json:"message_id"`
	Chat      tgChat `json:"chat"`
	Text      string `json:"text"`
}

type tgChat struct {
	ID int64 `json:"id"`
}

type tgUpdateResponse struct {
	OK     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

func startTelegramBot(repo repo.Repository) {
	token := os.Getenv("TOKEN_BOT")
	peerStr := os.Getenv("ADMIN_PEER_ID")
	if token == "" || peerStr == "" {
		log.Println("TG bot disabled: TOKEN_BOT or ADMIN_PEER_ID missing")
		return
	}
	adminID, _ := strconv.ParseInt(peerStr, 10, 64)
	offset := 0

	go func() {
		for {
			updates, err := tgGetUpdates(token, offset)
			if err != nil {
				log.Println("TG getUpdates error:", err)
				time.Sleep(2 * time.Second)
				continue
			}
			for _, u := range updates {
				offset = u.UpdateID + 1
				if u.CallbackQuery != nil {
					handleTGCallback(token, adminID, repo, u.CallbackQuery)
				}
				if u.Message != nil {
					handleTGMessage(token, adminID, u.Message)
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()
}


func handleTGMessage(token string, adminID int64, msg *tgMessage) {
	if msg.Chat.ID != adminID {
		return
	}
	if strings.TrimSpace(msg.Text) == "/log" {
		sendLogFile(token, adminID)
	}
}

func sendLogFile(token string, chatID int64) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", token)
	filePath := "./logs/app.log"
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	_ = writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	part, _ := writer.CreateFormFile("document", "app.log")
	_, _ = io.Copy(part, file)
	writer.Close()
	req, _ := http.NewRequest("POST", url, &b)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	_, _ = http.DefaultClient.Do(req)
}

func tgGetUpdates(token string, offset int) ([]tgUpdate, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=20&offset=%d", token, offset)
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var out tgUpdateResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

func handleTGCallback(token string, adminID int64, repo repo.Repository, cb *tgCallbackQuery) {
	if cb.Message == nil || cb.Message.Chat.ID != adminID {
		tgAnswerCallback(token, cb.ID, "Not allowed")
		return
	}
	parts := strings.Split(cb.Data, ":")
	if len(parts) != 2 {
		tgAnswerCallback(token, cb.ID, "Bad data")
		return
	}
	action := parts[0]
	id, _ := strconv.Atoi(parts[1])
	ticket, err := repo.GetPremiumTicket(context.Background(), id)
	if err != nil {
		tgAnswerCallback(token, cb.ID, "Ticket not found")
		return
	}
	switch action {
	case "approve":
		_ = repo.UpdatePremiumTicketStatus(context.Background(), id, "approved")
		until := time.Now().Add(30 * 24 * time.Hour)
		_ = repo.SetPremiumUntil(context.Background(), ticket.UserID, until)
		tgAnswerCallback(token, cb.ID, "Approved")
		tgEditMessage(token, cb.Message.Chat.ID, cb.Message.MessageID, fmt.Sprintf("✅ Approved ticket #%d", id))
	case "reject":
		_ = repo.UpdatePremiumTicketStatus(context.Background(), id, "rejected")
		tgAnswerCallback(token, cb.ID, "Rejected")
		tgEditMessage(token, cb.Message.Chat.ID, cb.Message.MessageID, fmt.Sprintf("❌ Rejected ticket #%d", id))
	default:
		tgAnswerCallback(token, cb.ID, "Unknown action")
	}
}

func tgAnswerCallback(token, id, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", token)
	payload := map[string]any{"callback_query_id": id, "text": text}
	b, _ := json.Marshal(payload)
	_, _ = http.Post(url, "application/json", bytes.NewReader(b))
}

func tgEditMessage(token string, chatID int64, messageID int, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", token)
	payload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	}
	b, _ := json.Marshal(payload)
	_, _ = http.Post(url, "application/json", bytes.NewReader(b))
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
		ticketID, err := userRepo.CreatePremiumTicket(r.Context(), userID, prof.Login, prof.Email)
		if err != nil {
			http.Error(w, "DB error", http.StatusInternalServerError)
			return
		}
		botToken := os.Getenv("TOKEN_BOT")
		peerStr := os.Getenv("ADMIN_PEER_ID")
		if botToken != "" && peerStr != "" {
			peerID, _ := strconv.ParseInt(peerStr, 10, 64)
			msg := fmt.Sprintf("Тикет на оплату:\n\nid: %d\nlogin: %s\nemail: %s\n\nticket_id: %d\nTime: %s", userID, prof.Login, prof.Email, ticketID, time.Now().Format("02.01.2006 : 15:04"))
			_ = sendTelegramTicket(botToken, peerID, msg, ticketID)
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

	
	// Premium tools (extra)
	premiumTools := secureApi.PathPrefix("/premium-tools").Subrouter()
	premiumTools.Use(premiumMiddleware(userRepo))
	premiumTools.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("PREMIUM tools request: %s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	})
	batchH := &pbatch.Handler{}
	autoH := &pauto.Handler{}
	recH := &preco.Handler{}
	impH := &pimport.Handler{}
	premiumTools.HandleFunc("/batch/beam", batchH.Beam).Methods("POST")
	premiumTools.HandleFunc("/auto/beam", autoH.Beam).Methods("POST")
	premiumTools.HandleFunc("/recommend/weld", recH.Weld).Methods("POST")
	premiumTools.HandleFunc("/import/beam", impH.Beam).Methods("POST")

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
	_ = godotenv.Load()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	db := auth.InitDB()
	defer db.Close()
	mux := mux.NewRouter()
	// Logging to file + stdout
	if err := os.MkdirAll("./logs", 0755); err == nil {
		if f, err := os.OpenFile("./logs/app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			log.SetOutput(io.MultiWriter(os.Stdout, f))
		}
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting server on :%s", port)
	HandleList(mux, db)
	startTelegramBot(repo.NewPostgresUserDB(db))
	handler := CORS(mux)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
