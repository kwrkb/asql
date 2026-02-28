package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateSQL(t *testing.T) {
	t.Run("successful response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected application/json content type")
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("expected Bearer test-key, got %q", r.Header.Get("Authorization"))
			}

			var req chatRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.Model != "test-model" {
				t.Errorf("expected model test-model, got %q", req.Model)
			}

			json.NewEncoder(w).Encode(chatResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{
					{Message: struct {
						Content string `json:"content"`
					}{Content: "SELECT * FROM users;"}},
				},
			})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "test-model", "test-key")
		sql, err := client.GenerateSQL(context.Background(), "sqlite", "CREATE TABLE users (id INT);", "show all users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sql != "SELECT * FROM users;" {
			t.Errorf("unexpected SQL: %q", sql)
		}
	})

	t.Run("no api key omits auth header", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				t.Errorf("expected no Authorization header, got %q", r.Header.Get("Authorization"))
			}
			json.NewEncoder(w).Encode(chatResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{
					{Message: struct {
						Content string `json:"content"`
					}{Content: "SELECT 1;"}},
				},
			})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "model", "")
		_, err := client.GenerateSQL(context.Background(), "sqlite", "", "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("API error status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "model", "")
		_, err := client.GenerateSQL(context.Background(), "sqlite", "", "test")
		if err == nil {
			t.Error("expected error for 500 status")
		}
	})

	t.Run("empty choices", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(chatResponse{})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "model", "")
		_, err := client.GenerateSQL(context.Background(), "sqlite", "", "test")
		if err == nil {
			t.Error("expected error for empty choices")
		}
	})
}

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no fences", "SELECT 1;", "SELECT 1;"},
		{"sql fences", "```sql\nSELECT 1;\n```", "SELECT 1;"},
		{"plain fences", "```\nSELECT 1;\n```", "SELECT 1;"},
		{"with whitespace", "  ```sql\n  SELECT 1;\n  ```  ", "SELECT 1;"},
		{"no content after fence", "```", ""},
		{"multiline", "```sql\nSELECT *\nFROM users\nWHERE id = 1;\n```", "SELECT *\nFROM users\nWHERE id = 1;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripCodeFences(tt.input)
			if got != tt.want {
				t.Errorf("stripCodeFences(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
