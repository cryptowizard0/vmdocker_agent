package openclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func TestApply(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "data": "pong"})
		case "/v1/tools/invoke":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
				"data":   "handled:" + payload["action"],
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer gateway.Close()

	t.Setenv("OPENCLAW_GATEWAY_URL", gateway.URL)
	t.Setenv("OPENCLAW_TIMEOUT_MS", "1000")

	rt, err := NewRuntime()
	if err != nil {
		t.Fatalf("new openclaw runtime failed: %v", err)
	}

	t.Run("meta action has higher priority", func(t *testing.T) {
		res, err := rt.Apply("target-1", vmmSchema.Meta{Action: "Ping", Sequence: 9}, map[string]string{"action": "Execute"})
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}
		if res.Data != "pong" {
			t.Fatalf("expected pong, got %s", res.Data)
		}
		output, ok := res.Output.(map[string]interface{})
		if !ok {
			t.Fatalf("expected output map, got %T", res.Output)
		}
		if output["action"] != ActionPing {
			t.Fatalf("expected action Ping, got %v", output["action"])
		}
	})

	t.Run("fallback to query action", func(t *testing.T) {
		res, err := rt.Apply("target-2", vmmSchema.Meta{Sequence: 10}, map[string]string{})
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}
		if res.Data != "handled:Query" {
			t.Fatalf("expected handled:Query, got %s", res.Data)
		}
		output, ok := res.Output.(map[string]interface{})
		if !ok {
			t.Fatalf("expected output map, got %T", res.Output)
		}
		if output["action"] != ActionQuery {
			t.Fatalf("expected action Query, got %v", output["action"])
		}
	})

	if err := rt.client.Close(context.Background()); err != nil {
		t.Fatalf("close runtime client failed: %v", err)
	}
}
