package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	schema "github.com/cryptowizard0/vmdocker_agent/runtime/openclaw/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func TestNewRuntimeOpenclaw(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","data":"pong"}`))
			return
		}
		if r.URL.Path == "/tools/invoke" {
			var req schema.ToolInvokeRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Tool == "sessions_create" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok","data":{"sessionId":"runtime-sess-1"}}`))
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":"error"}`))
	}))
	defer gateway.Close()

	t.Setenv("RUNTIME_TYPE", RuntimeTypeOpenclaw)
	t.Setenv("OPENCLAW_GATEWAY_URL", gateway.URL)
	t.Setenv("OPENCLAW_TIMEOUT_MS", "1000")

	rt, err := New(vmmSchema.Env{}, "", "", nil)
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}
	if rt == nil || rt.vm == nil {
		t.Fatalf("runtime vm is nil")
	}
}
