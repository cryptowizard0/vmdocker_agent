package runtime

import (
	"net/http"
	"net/http/httptest"
	"testing"

	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func TestNewRuntimeOpenclaw(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","data":"pong"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","data":"done"}`))
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
