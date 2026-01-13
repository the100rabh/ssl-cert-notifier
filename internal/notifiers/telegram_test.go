package notifiers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/the100rabh/ssl-cert-notifier/internal/config"
)

func TestTelegramNotifier_Send(t *testing.T) {
	testBotToken := "test_bot_token"
	testChatID := "test_chat_id"
	testSubject := "Test Subject"
	testBody := "Test Body"

	t.Run("Send success", func(t *testing.T) {
		// Mock Telegram API server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("Expected POST request, got %s", r.Method)
			}
			if !strings.Contains(r.URL.Path, fmt.Sprintf("/bot%s/sendMessage", testBotToken)) {
				t.Errorf("Expected URL path to contain bot token, got %s", r.URL.Path)
			}

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)

			if payload["chat_id"] != testChatID {
				t.Errorf("Expected chat_id %s, got %s", testChatID, payload["chat_id"])
			}
			expectedText := fmt.Sprintf("Subject: %s\n\n%s", testSubject, testBody)
			if payload["text"] != expectedText {
				t.Errorf("Expected text %s, got %s", expectedText, payload["text"])
			}

			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"ok":true,"result":{"message_id":1,"from":{"id":123,"is_bot":true,"first_name":"test"},"chat":{"id":123,"first_name":"test","type":"private"},"date":123,"text":"hello"}}`)
		}))
		defer server.Close()

		notifier, err := NewTelegramNotifier(config.Notifier{BotToken: testBotToken, ChatID: testChatID})
		if err != nil {
			t.Fatalf("Failed to create notifier: %v", err)
		}
		notifier.telegramAPIBaseURL = server.URL // Override for testing

		err = notifier.Send(testSubject, testBody)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Send API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`)
		}))
		defer server.Close()

		nnotifier, err := NewTelegramNotifier(config.Notifier{BotToken: testBotToken, ChatID: testChatID})
		if err != nil {
			t.Fatalf("Failed to create notifier: %v", err)
		}
		nnotifier.telegramAPIBaseURL = server.URL // Override for testing

		err = nnotifier.Send(testSubject, testBody)
		if err == nil {
			t.Fatal("Expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "400 Bad Request") || !strings.Contains(err.Error(), "chat not found") {
			t.Errorf("Expected specific API error, got %v", err)
		}
	})

	t.Run("Send network error", func(t *testing.T) {
		nnotifier, err := NewTelegramNotifier(config.Notifier{BotToken: testBotToken, ChatID: testChatID})
		if err != nil {
			t.Fatalf("Failed to create notifier: %v", err)
		}
		// Point to an unreachable address and set a short timeout to force network error
		nnotifier.telegramAPIBaseURL = "http://127.0.0.1:0" // Unreachable
		nnotifier.Client.Timeout = 1 * time.Millisecond     // Short timeout

		err = nnotifier.Send(testSubject, testBody)
		if err == nil {
			t.Fatal("Expected a network error, got nil")
		}
		if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "connection refused") && !strings.Contains(err.Error(), "dial") {
			t.Errorf("Expected a network error, got %v", err)
		}
	})
}
