package main

import (
	"Vertex/internal/auth"
	"Vertex/internal/repo"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	Data    string   `json:"data"`
	Message *Message `json:"message"`
}

type UpdateResponse struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

func main() {
	token := os.Getenv("TOKEN_BOT")
	peerStr := os.Getenv("ADMIN_PEER_ID")
	if token == "" || peerStr == "" {
		log.Fatal("TOKEN_BOT or ADMIN_PEER_ID missing")
	}
	adminID, _ := strconv.ParseInt(peerStr, 10, 64)

	db := auth.InitDB()
	defer db.Close()
	repo := repo.NewPostgresUserDB(db)

	offset := 0
	for {
		updates, err := getUpdates(token, offset)
		if err != nil {
			log.Println("getUpdates error:", err)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.CallbackQuery != nil {
				handleCallback(token, adminID, repo, u.CallbackQuery)
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func handleCallback(token string, adminID int64, repo *repo.PostgresUserRepository, cb *CallbackQuery) {
	if cb.Message == nil || cb.Message.Chat.ID != adminID {
		answerCallback(token, cb.ID, "Not allowed")
		return
	}
	parts := strings.Split(cb.Data, ":")
	if len(parts) != 2 {
		answerCallback(token, cb.ID, "Bad data")
		return
	}
	action := parts[0]
	id, _ := strconv.Atoi(parts[1])
	ticket, err := repo.GetPremiumTicket(context.Background(), id)
	if err != nil {
		answerCallback(token, cb.ID, "Ticket not found")
		return
	}

	switch action {
	case "approve":
		_ = repo.UpdatePremiumTicketStatus(context.Background(), id, "approved")
		until := time.Now().Add(30 * 24 * time.Hour)
		_ = repo.SetPremiumUntil(context.Background(), ticket.UserID, until)
		answerCallback(token, cb.ID, "Approved")
		editMessage(token, cb.Message.Chat.ID, cb.Message.MessageID, fmt.Sprintf("✅ Approved ticket #%d", id))
	case "reject":
		_ = repo.UpdatePremiumTicketStatus(context.Background(), id, "rejected")
		answerCallback(token, cb.ID, "Rejected")
		editMessage(token, cb.Message.Chat.ID, cb.Message.MessageID, fmt.Sprintf("❌ Rejected ticket #%d", id))
	default:
		answerCallback(token, cb.ID, "Unknown action")
	}
}

func getUpdates(token string, offset int) ([]Update, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=20&offset=%d", token, offset)
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var out UpdateResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

func answerCallback(token, id, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", token)
	payload := map[string]any{"callback_query_id": id, "text": text}
	b, _ := json.Marshal(payload)
	_, _ = http.Post(url, "application/json", strings.NewReader(string(b)))
}

func editMessage(token string, chatID int64, messageID int, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", token)
	payload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	}
	b, _ := json.Marshal(payload)
	_, _ = http.Post(url, "application/json", strings.NewReader(string(b)))
}

