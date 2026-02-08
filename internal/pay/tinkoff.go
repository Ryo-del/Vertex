package pay //Dont't work now

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Client struct {
	TerminalKey string
	Password    string
	BaseURL     string
	HTTPClient  *http.Client
}

type InitRequest struct {
	TerminalKey     string `json:"TerminalKey"`
	Amount          int64  `json:"Amount"`
	OrderID         string `json:"OrderId"`
	Description     string `json:"Description,omitempty"`
	SuccessURL      string `json:"SuccessURL,omitempty"`
	FailURL         string `json:"FailURL,omitempty"`
	NotificationURL string `json:"NotificationURL,omitempty"`
	CustomerKey     string `json:"CustomerKey,omitempty"`
}

type InitResponse struct {
	Success    bool   `json:"Success"`
	Status     string `json:"Status"`
	PaymentID  string `json:"PaymentId"`
	PaymentURL string `json:"PaymentURL"`
	Message    string `json:"Message,omitempty"`
	Details    string `json:"Details,omitempty"`
}

type StateRequest struct {
	TerminalKey string `json:"TerminalKey"`
	PaymentID   string `json:"PaymentId"`
}

type StateResponse struct {
	Success   bool   `json:"Success"`
	Status    string `json:"Status"`
	PaymentID string `json:"PaymentId"`
	OrderID   string `json:"OrderId"`
	Message   string `json:"Message,omitempty"`
	Details   string `json:"Details,omitempty"`
}

func NewClient(terminalKey, password string) *Client {
	return &Client{
		TerminalKey: terminalKey,
		Password:    password,
		BaseURL:     "https://securepay.tinkoff.ru/v2",
		HTTPClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Init(req InitRequest) (InitResponse, error) {
	req.TerminalKey = c.TerminalKey
	payload, err := signRequest(c.Password, req)
	if err != nil {
		return InitResponse{}, err
	}
	var resp InitResponse
	if err := c.postJSON("/Init", payload, &resp); err != nil {
		return InitResponse{}, err
	}
	if !resp.Success {
		return resp, fmt.Errorf("init failed: %s %s", resp.Message, resp.Details)
	}
	return resp, nil
}

func (c *Client) GetState(paymentID string) (StateResponse, error) {
	req := StateRequest{TerminalKey: c.TerminalKey, PaymentID: paymentID}
	payload, err := signRequest(c.Password, req)
	if err != nil {
		return StateResponse{}, err
	}
	var resp StateResponse
	if err := c.postJSON("/GetState", payload, &resp); err != nil {
		return StateResponse{}, err
	}
	if !resp.Success {
		return resp, fmt.Errorf("get state failed: %s %s", resp.Message, resp.Details)
	}
	return resp, nil
}

func (c *Client) VerifyToken(data map[string]any, token string) bool {
	expected := makeToken(c.Password, data)
	return strings.EqualFold(expected, token)
}

func (c *Client) postJSON(path string, payload map[string]any, out any) error {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(out)
}

func signRequest(password string, req any) (map[string]any, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	token := makeToken(password, m)
	m["Token"] = token
	return m, nil
}

func makeToken(password string, m map[string]any) string {
	keys := make([]string, 0, len(m)+1)
	for k := range m {
		if strings.EqualFold(k, "Token") {
			continue
		}
		keys = append(keys, k)
	}
	keys = append(keys, "Password")
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		if k == "Password" {
			b.WriteString(password)
			continue
		}
		v := m[k]
		switch t := v.(type) {
		case string:
			b.WriteString(t)
		case float64:
			b.WriteString(fmt.Sprintf("%.0f", t))
		case int:
			b.WriteString(fmt.Sprintf("%d", t))
		case int64:
			b.WriteString(fmt.Sprintf("%d", t))
		case bool:
			if t {
				b.WriteString("true")
			} else {
				b.WriteString("false")
			}
		default:
			b.WriteString(fmt.Sprintf("%v", t))
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
