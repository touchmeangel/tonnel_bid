package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type TGLogger struct {
	Token  string
	ChatID int64
	Client *http.Client
}

type sendMessagePayload struct {
	Text                  string                `json:"text"`
	ChatID                int64                 `json:"chat_id"`
	ParseMode             string                `json:"parse_mode"`
	ReplyToMessageID      int64                 `json:"reply_to_message_id,omitempty"`
	DisableWebPagePreview bool                  `json:"disable_web_page_preview"`
	ReplyMarkup           *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

type InlineKeyboardButton struct {
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type ReplyMarkup struct {
}

type tgErrorResponse struct {
	Ok          bool `json:"ok"`
	ErrorCode   int  `json:"error_code"`
	Description string
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func NewLogger(token string, chatID int64) *TGLogger {
	return &TGLogger{
		Token:  token,
		ChatID: chatID,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TGLogger) SendMessage(ctx context.Context, message string, wait bool, replyTo *int64, markup *InlineKeyboardMarkup) error {
	payload := sendMessagePayload{
		Text:                  message,
		ChatID:                t.ChatID,
		ParseMode:             "HTML",
		DisableWebPagePreview: true,
		ReplyMarkup:           markup,
	}
	if replyTo != nil {
		payload.ReplyToMessageID = *replyTo
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response body
	respBody, _ := io.ReadAll(resp.Body)

	// Handle FloodWait (429)
	if resp.StatusCode == http.StatusTooManyRequests && wait {
		var errResp tgErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Parameters.RetryAfter > 0 {
			log.Printf("FLOOD WAIT 429: retrying after %d seconds...\n", errResp.Parameters.RetryAfter)
			time.Sleep(time.Duration(errResp.Parameters.RetryAfter) * time.Second)
			// Retry only once
			return t.SendMessage(ctx, message, false, replyTo, markup)
		}
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram API error %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
