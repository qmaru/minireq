package minireq

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestServerWithHandler(handler http.Handler) *httptest.Server {
	return httptest.NewServer(handler)
}

func newGetServer() *httptest.Server {
	return newTestServerWithHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte(`{"ok":true}`))
	}))
}

func newPostJSONServer() *httptest.Server {
	return newTestServerWithHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte(`{"ok":true}`))
	}))
}

func newMinireqClient() *HttpClient {
	client := NewClient()
	client.SetMaxIdleConns(100)
	client.SetMaxIdleConnsPerHost(100)
	client.SetIdleConnTimeout(60)
	return client
}

func newStdClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     60 * time.Second,
	}
	return &http.Client{
		Transport: transport,
	}
}

func BenchmarkGetParallel(b *testing.B) {
	srv := newGetServer()
	defer srv.Close()

	url := srv.URL

	minireq := newMinireqClient()
	stdClient := newStdClient()

	b.Run("minireq", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				res, err := minireq.Get(url)
				if err != nil {
					b.Fatalf("minireq Get request failed: %v", err)
				}
				if err := res.Close(); err != nil {
					b.Fatalf("minireq Close failed: %v", err)
				}
			}
		})
	})

	b.Run("standard", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				resp, err := stdClient.Get(url)
				if err != nil {
					b.Fatalf("std http Get request failed: %v", err)
				}
				_ = resp.Body.Close()
			}
		})
	})
}

func BenchmarkPostJSONParallel(b *testing.B) {
	srv := newPostJSONServer()
	defer srv.Close()

	url := srv.URL

	minireq := newMinireqClient()
	stdClient := newStdClient()

	b.Run("minireq", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				res, err := minireq.Post(url, JSONData{"foo": "bar"})
				if err != nil {
					b.Fatalf("minireq Post json failed: %v", err)
				}
				if err := res.Close(); err != nil {
					b.Fatalf("minireq Close failed: %v", err)
				}
			}
		})
	})

	b.Run("standard", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				jsonPayload := map[string]string{"foo": "bar"}
				jsonBytes, err := json.Marshal(jsonPayload)
				if err != nil {
					b.Fatalf("failed to marshal json: %v", err)
				}
				resp, err := stdClient.Post(url, "application/json", bytes.NewReader(jsonBytes))
				if err != nil {
					b.Fatalf("std http Post json failed: %v", err)
				}
				_ = resp.Body.Close()
			}
		})
	})
}
