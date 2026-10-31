package httpapi_test

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/health"
	"bidora/backend/internal/httpapi"
	"github.com/gofiber/fiber/v3"
)

func TestHealthBoundary(t *testing.T) {
	for _, tt := range []struct {
		name, method, path, body string
		ready                    bool
		status                   int
		code                     string
	}{
		{"live before ready", "GET", "/health/live", "", false, 200, ""},
		{"not initialized", "GET", "/health/ready", "", false, 503, "SERVICE_NOT_READY"},
		{"ready", "GET", "/health/ready", "", true, 200, ""},
		{"query", "GET", "/health/live?dsn=secret", "", true, 400, "INVALID_REQUEST"},
		{"body", "GET", "/health/ready", "secret", true, 400, "INVALID_REQUEST"},
		{"method", "POST", "/health/ready", "", true, 405, "METHOD_NOT_ALLOWED"},
		{"head", "HEAD", "/health/ready", "", true, 405, ""},
		{"unknown", "GET", "/v1/private", "", true, 404, "NOT_FOUND"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := &health.State{}
			if tt.ready {
				state.Start()
			}
			app := httpapi.New(state)
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("X-Request-ID", "attacker-controlled-secret")
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tt.status {
				t.Fatalf("status %d, want %d", res.StatusCode, tt.status)
			}
			if res.Header.Get("Cache-Control") != "no-store" {
				t.Fatal("missing no-store")
			}
			id := res.Header.Get("X-Request-ID")
			if len(id) != 36 || id[14] != '4' {
				t.Fatalf("invalid request id %q", id)
			}
			if tt.status == 405 && res.Header.Get("Allow") != "GET" {
				t.Fatal("missing allow")
			}
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), "secret") {
				t.Fatal("private input leaked")
			}
			if tt.method == "HEAD" {
				return
			} // HTTP HEAD correctly omits response bytes.
			var envelope map[string]json.RawMessage
			if err = json.Unmarshal(body, &envelope); err != nil {
				t.Fatal(err)
			}
			if tt.status == 200 {
				var v struct {
					Meta struct {
						TraceID string `json:"trace_id"`
					} `json:"meta"`
				}
				if err = json.Unmarshal(body, &v); err != nil {
					t.Fatal(err)
				}
				if v.Meta.TraceID != id || len(envelope) != 2 {
					t.Fatal("invalid success envelope")
				}
			} else {
				var v struct {
					Error struct {
						TraceID string `json:"trace_id"`
						Code    string `json:"code"`
					} `json:"error"`
				}
				if err = json.Unmarshal(body, &v); err != nil {
					t.Fatal(err)
				}
				if v.Error.TraceID != id || v.Error.Code != tt.code || len(envelope) != 1 {
					t.Fatal("invalid error envelope")
				}
			}
		})
	}
}

// Fiber's in-memory Test reports transport parsing errors directly. Use a real
// listener to assert the actual oversized-request wire response instead.
func TestOversizedRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.New(&health.State{})
	done := make(chan error, 1)
	go func() { done <- app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() {
		if err := app.ShutdownWithTimeout(time.Second); err != nil {
			t.Error(err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(2 * time.Second):
			t.Error("listener did not stop")
		}
	})
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Post("http://"+listener.Addr().String()+"/health/ready", "text/plain", strings.NewReader(strings.Repeat("s", 4097)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 413 {
		t.Fatalf("status %d, want 413", res.StatusCode)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			TraceID string `json:"trace_id"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "REQUEST_TOO_LARGE" || body.Error.TraceID != res.Header.Get("X-Request-ID") {
		t.Fatal("invalid safe error")
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("error may be cached")
	}
}

func TestDrainAndRequestIDs(t *testing.T) {
	state := &health.State{}
	state.Start()
	app := httpapi.New(state)
	first, err := app.Test(httptest.NewRequest("GET", "/health/ready", nil))
	if err != nil {
		t.Fatal(err)
	}
	first.Body.Close()
	if first.StatusCode != 200 {
		t.Fatal("not ready")
	}
	state.Drain()
	second, err := app.Test(httptest.NewRequest("GET", "/health/ready", nil))
	if err != nil {
		t.Fatal(err)
	}
	second.Body.Close()
	if second.StatusCode != 503 {
		t.Fatal("drain accepted traffic")
	}
	if first.Header.Get("X-Request-ID") == second.Header.Get("X-Request-ID") {
		t.Fatal("reused request id")
	}
	live, err := app.Test(httptest.NewRequest("GET", "/health/live", nil))
	if err != nil {
		t.Fatal(err)
	}
	live.Body.Close()
	if live.StatusCode != 200 {
		t.Fatal("draining process should remain live")
	}
}
