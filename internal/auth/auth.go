package auth

import (
	repo "Vertex/internal/repo"
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

type contextKey string

const userIDKey contextKey = "userID"

type Authenv struct {
	JWTkey []byte
	Repo   repo.Repository
}

type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  sync.RWMutex
	r   rate.Limit
	b   int
}
type Loginrequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Registerrequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		r:   r,
		b:   b,
	}
}
func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}
	return limiter
}

// Rate limiting middleware
func (i *IPRateLimiter) LimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use remote address as rate limiting key
		ip := r.RemoteAddr

		limiter := i.getLimiter(ip)
		if !limiter.Allow() {
			http.Error(w, "Too Many Requests. Try again later.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
func (env *Authenv) RedirectIfLoggedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err == nil && env.isValidToken(cookie.Value) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (env *Authenv) isValidToken(tokenString string) bool {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// verify signing method and return key
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return env.JWTkey, nil
	})

	if err != nil {
		log.Println("Ошибка парсинга токена:", err)
		return false
	}

	return token.Valid
}
func (env *Authenv) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
			return
		}

		// Parse JWT and validate
		token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
			return env.JWTkey, nil
		})

		if err != nil || !token.Valid {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
			return
		}

		userIDValue, ok := claims["user_id"]
		if !ok {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
			return
		}
		userIDFloat, ok := userIDValue.(float64)
		if !ok {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
			return
		}

		loginValue, ok := claims["login"]
		if !ok {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
			return
		}
		login, ok := loginValue.(string)
		if !ok || login == "" {
			http.Redirect(w, r, "/auth/", http.StatusSeeOther)
			return
		}

		// Кладем в контекст оба значения
		ctx := context.WithValue(r.Context(), userIDKey, int(userIDFloat))
		ctx = context.WithValue(ctx, "userID", int(userIDFloat))
		ctx = context.WithValue(ctx, "userLogin", login)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (env *Authenv) addCookie(w http.ResponseWriter, userID int, login string) {
	// Create JWT cookie
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"login":   login,
		"exp":     time.Now().Add(30 * 24 * time.Hour).Unix(), // Токен на месяц
	})
	tokenString, err := token.SignedString(env.JWTkey)
	if err != nil {
		log.Println("Ошибка создания токена:", err)
		return
	}
	expiration := time.Now().Add(30 * 24 * time.Hour)
	cookie := http.Cookie{
		Name:     "session_token",
		Value:    tokenString, // Обычно это UUID или JWT
		Expires:  expiration,
		Path:     "/",
		HttpOnly: true, // Важно! JS не сможет украсть эту куку
		Secure:   true, // Работает только через HTTPS
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, &cookie)
}
func InitDB() *sql.DB {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "user=postgres dbname=postgres password=password sslmode=disable"
	}
	if !strings.Contains(connStr, "sslmode=") {
		if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
			connStr = connStr + "?sslmode=require"
		} else {
			connStr = connStr + " sslmode=require"
		}
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Ошибка конфигурации БД:", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.Ping(); err != nil {
		log.Fatal("База не отвечает:", err)
	}
	return db
}
func (env *Authenv) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req Registerrequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	req.Login = strings.TrimSpace(req.Login)
	req.Email = strings.TrimSpace(req.Email)
	if req.Login == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "Login, email and password required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 6 {
		http.Error(w, "Password too короткий", http.StatusBadRequest)
		return
	}

	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Error hashing password", http.StatusInternalServerError)
		return
	}
	id, err := env.Repo.CreateUser(r.Context(), req.Login, req.Email, hashedPassword)
	if err != nil {
		log.Printf("CreateUser Error: %v", err)
		http.Error(w, "User already exists or DB error", http.StatusConflict)
		return
	}

	env.addCookie(w, id, req.Login)
	// Явно говорим, что ресурс создан
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Registration successful"))
}

func (env *Authenv) AuthHandler(w http.ResponseWriter, r *http.Request) {
	// Реализация аутентификации пользователя
	var req Loginrequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	req.Login = strings.TrimSpace(req.Login)
	if req.Login == "" || req.Password == "" {
		http.Error(w, "Login and password required", http.StatusBadRequest)
		return
	}

	id, storedHash, err := env.Repo.GetBylogin(r.Context(), req.Login)
	if err != nil {
		log.Printf("GetBylogin Error: %v", err)
		http.Error(w, "User already exists or DB error", http.StatusConflict)
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password))
	if err != nil {
		http.Error(w, "Invalid login or password", http.StatusUnauthorized)
		return
	}
	env.addCookie(w, id, req.Login)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Authentication successful"))
}
