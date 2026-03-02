package openclaw

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	schema "github.com/cryptowizard0/vmdocker_agent/runtime/openclaw/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func TestNewAndApply(t *testing.T) {
	const token = "test-gateway-token"

	createCalled := false
	sendCalled := false
	seenCommand := ""

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "data": "pong"})
		case "/tools/invoke":
			if got := r.Header.Get("x-gateway-token"); got != token {
				t.Fatalf("expected x-gateway-token %q, got %q", token, got)
			}

			var req schema.ToolInvokeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode invoke request failed: %v", err)
			}

			switch req.Tool {
			case DefaultToolCreateSession:
				createCalled = true
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "ok",
					"data":   map[string]interface{}{"sessionId": "sess-1"},
				})
			case DefaultToolSendSession:
				sendCalled = true
				if req.SessionKey != "sess-1" {
					t.Fatalf("expected top-level sessionKey sess-1, got %q", req.SessionKey)
				}
				if req.Args["sessionKey"] != "sess-1" {
					t.Fatalf("expected args.sessionKey sess-1, got %v", req.Args["sessionKey"])
				}
				text, _ := req.Args["message"].(string)
				seenCommand = text

				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "ok",
					"result": map[string]interface{}{"reply": "handled"},
				})
			default:
				http.Error(w, "unsupported tool", http.StatusBadRequest)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer gateway.Close()

	t.Setenv("OPENCLAW_GATEWAY_URL", gateway.URL)
	t.Setenv("OPENCLAW_TIMEOUT_MS", "1000")
	t.Setenv("OPENCLAW_GATEWAY_TOKEN", token)
	t.Setenv("OPENCLAW_SESSION_TITLE", "runtime-test")

	rt, err := New()
	if err != nil {
		t.Fatalf("new openclaw runtime failed: %v", err)
	}
	if rt.sessionID() != "sess-1" {
		t.Fatalf("expected session id sess-1, got %q", rt.sessionID())
	}

	res, err := rt.Apply("target-1", vmmSchema.Meta{Action: "Execute", Sequence: 9}, map[string]string{
		"Command":   "run this",
		"Reference": "9",
	})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if res.Data != "handled" {
		t.Fatalf("expected handled, got %q", res.Data)
	}

	if !createCalled {
		t.Fatalf("expected sessions_create to be called")
	}
	if !sendCalled {
		t.Fatalf("expected sessions_send to be called")
	}
	if seenCommand != "run this" {
		t.Fatalf("expected command run this, got %q", seenCommand)
	}
}
