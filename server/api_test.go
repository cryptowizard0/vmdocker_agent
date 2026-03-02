package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	vmdockerSchema "github.com/cryptowizard0/vmdocker/vmdocker/schema"
	openclawSchema "github.com/cryptowizard0/vmdocker_agent/runtime/openclaw/schema"
	"github.com/gin-gonic/gin"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)

	s := New(0)
	s.engine = gin.New()

	engine := s.engine.Group("/vmm")
	engine.POST("/health", s.health)
	engine.POST("/apply", s.apply)
	engine.POST("/spawn", s.spawn)

	return s
}

func performJSONRequest(t *testing.T, s *Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body failed: %v", err)
		}
		reader = bytes.NewReader(data)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	return w
}

func performRawJSONRequest(s *Server, method, path, raw string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	return w
}

func TestHealth(t *testing.T) {
	s := setupTestServer(t)

	w := performJSONRequest(t, s, http.MethodPost, "/vmm/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if res["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", res["status"])
	}
}

func TestApplyWithoutSpawn(t *testing.T) {
	s := setupTestServer(t)

	w := performJSONRequest(t, s, http.MethodPost, "/vmm/apply", vmdockerSchema.ApplyRequest{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if res["msg"] != "runtime is nil" {
		t.Fatalf("expected msg runtime is nil, got %q", res["msg"])
	}
}

func TestSpawnAndApply(t *testing.T) {
	t.Setenv("RUNTIME_TYPE", "test")
	s := setupTestServer(t)

	spawnReq := vmdockerSchema.SpawnRequest{
		Pid:    "pid-1",
		Owner:  "owner-1",
		CuAddr: "cu-1",
		Data:   []byte{},
		Tags:   nil,
		Evn:    vmmSchema.Env{},
	}
	w := performJSONRequest(t, s, http.MethodPost, "/vmm/spawn", spawnReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected spawn status 200, got %d: %s", w.Code, w.Body.String())
	}

	applyReq := vmdockerSchema.ApplyRequest{
		From: "target-1",
		Meta: vmmSchema.Meta{
			Action:   "Ping",
			Sequence: 7,
		},
		Params: map[string]string{
			"Action":    "Ping",
			"Reference": "7",
		},
	}
	w = performJSONRequest(t, s, http.MethodPost, "/vmm/apply", applyReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected apply status 200, got %d: %s", w.Code, w.Body.String())
	}

	var res struct {
		Status string `json:"status"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal apply response failed: %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("expected status ok, got %q", res.Status)
	}

	var out vmmSchema.Result
	if err := json.Unmarshal([]byte(res.Result), &out); err != nil {
		t.Fatalf("unmarshal result payload failed: %v", err)
	}
	if out.Data != "Pong" {
		t.Fatalf("expected result data Pong, got %q", out.Data)
	}
	if len(out.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out.Messages))
	}
	if out.Messages[0].Target != "target-1" {
		t.Fatalf("expected message target target-1, got %q", out.Messages[0].Target)
	}
}

func TestSpawnAndApplyOpenclaw(t *testing.T) {
	createCalled := false
	sendCalled := false

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "data": "pong"})
		case "/tools/invoke":
			var req openclawSchema.ToolInvokeRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			switch req.Tool {
			case "sessions_create":
				createCalled = true
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "ok",
					"data":   map[string]interface{}{"sessionId": "sess-api-1"},
				})
			case "sessions_send":
				sendCalled = true
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "ok",
					"data":   map[string]interface{}{"message": "handled:Execute"},
				})
			default:
				http.Error(w, "unsupported tool", http.StatusBadRequest)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer gateway.Close()

	t.Setenv("RUNTIME_TYPE", "openclaw")
	t.Setenv("OPENCLAW_GATEWAY_URL", gateway.URL)
	t.Setenv("OPENCLAW_TIMEOUT_MS", "1000")

	s := setupTestServer(t)

	spawnReq := vmdockerSchema.SpawnRequest{
		Pid:    "pid-openclaw",
		Owner:  "owner-1",
		CuAddr: "cu-1",
		Data:   []byte{},
		Tags:   nil,
		Evn:    vmmSchema.Env{},
	}
	w := performJSONRequest(t, s, http.MethodPost, "/vmm/spawn", spawnReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected spawn status 200, got %d: %s", w.Code, w.Body.String())
	}

	applyReq := vmdockerSchema.ApplyRequest{
		From: "target-oc-1",
		Meta: vmmSchema.Meta{
			Action:   "Execute",
			Sequence: 12,
		},
		Params: map[string]string{
			"Command":   "hello",
			"Reference": "12",
		},
	}
	w = performJSONRequest(t, s, http.MethodPost, "/vmm/apply", applyReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected apply status 200, got %d: %s", w.Code, w.Body.String())
	}

	var res struct {
		Status string `json:"status"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal apply response failed: %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("expected status ok, got %q", res.Status)
	}

	var out vmmSchema.Result
	if err := json.Unmarshal([]byte(res.Result), &out); err != nil {
		t.Fatalf("unmarshal result payload failed: %v", err)
	}
	if out.Data != "handled:Execute" {
		t.Fatalf("expected result data handled:Execute, got %q", out.Data)
	}
	if len(out.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out.Messages))
	}
	if out.Messages[0].Target != "target-oc-1" {
		t.Fatalf("expected message target target-oc-1, got %q", out.Messages[0].Target)
	}
	if !createCalled {
		t.Fatalf("expected sessions_create to be called")
	}
	if !sendCalled {
		t.Fatalf("expected sessions_send to be called")
	}
}

func TestSpawnOpenclawSetupOnSpawn(t *testing.T) {
	createCalled := false
	setModelCalled := false
	gatewayPatchCalled := false

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "data": "pong"})
		case "/tools/invoke":
			var req openclawSchema.ToolInvokeRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			switch req.Tool {
			case "sessions_create":
				createCalled = true
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "ok",
					"data":   map[string]interface{}{"sessionId": "sess-setup-1"},
				})
			case "session_status":
				setModelCalled = true
				if req.SessionKey != "sess-setup-1" {
					t.Fatalf("expected top-level sessionKey sess-setup-1, got %q", req.SessionKey)
				}
				if req.Args["sessionKey"] != "sess-setup-1" {
					t.Fatalf("expected args.sessionKey sess-setup-1, got %v", req.Args["sessionKey"])
				}
				if req.Args["model"] != "gpt-4o" {
					t.Fatalf("expected model gpt-4o, got %v", req.Args["model"])
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "ok",
					"data":   "model-updated",
				})
			case "gateway":
				gatewayPatchCalled = true
				act, _ := req.Args["action"].(string)
				if act != "config.patch" {
					t.Fatalf("expected action config.patch, got %q", act)
				}
				raw, _ := req.Args["raw"].(string)
				if raw == "" {
					t.Fatalf("expected raw json patch string")
				}
				var patch map[string]interface{}
				if err := json.Unmarshal([]byte(raw), &patch); err != nil {
					t.Fatalf("decode raw patch failed: %v", err)
				}
				channels, _ := patch["channels"].(map[string]interface{})
				tg, _ := channels["telegram"].(map[string]interface{})
				if tg["botToken"] != "tg-token-setup-1" {
					t.Fatalf("expected botToken tg-token-setup-1, got %v", tg["botToken"])
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "ok",
					"data":   "patched",
				})
			default:
				http.Error(w, "unsupported tool", http.StatusBadRequest)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer gateway.Close()

	t.Setenv("RUNTIME_TYPE", "openclaw")
	t.Setenv("OPENCLAW_GATEWAY_URL", gateway.URL)
	t.Setenv("OPENCLAW_TIMEOUT_MS", "1000")

	s := setupTestServer(t)

	spawnReq := map[string]interface{}{
		"pid":     "pid-openclaw-setup",
		"owner":   "owner-1",
		"cu_addr": "cu-1",
		"data":    "",
		"tags":    []interface{}{},
		"env":     map[string]interface{}{},
		"params": map[string]string{
			"model":    "gpt-4o",
			"botToken": "tg-token-setup-1",
		},
	}
	w := performJSONRequest(t, s, http.MethodPost, "/vmm/spawn", spawnReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected spawn status 200, got %d: %s", w.Code, w.Body.String())
	}

	if !createCalled {
		t.Fatalf("expected sessions_create to be called")
	}
	if !setModelCalled {
		t.Fatalf("expected session_status to be called")
	}
	if !gatewayPatchCalled {
		t.Fatalf("expected gateway config.patch to be called")
	}
}

func TestSpawnTwice(t *testing.T) {
	t.Setenv("RUNTIME_TYPE", "test")
	s := setupTestServer(t)

	spawnReq := vmdockerSchema.SpawnRequest{
		Pid:    "pid-1",
		Owner:  "owner-1",
		CuAddr: "cu-1",
	}
	w := performJSONRequest(t, s, http.MethodPost, "/vmm/spawn", spawnReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected first spawn status 200, got %d: %s", w.Code, w.Body.String())
	}

	w = performJSONRequest(t, s, http.MethodPost, "/vmm/spawn", spawnReq)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected second spawn status 400, got %d", w.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if res["msg"] != "runtime is not nil" {
		t.Fatalf("expected msg runtime is not nil, got %q", res["msg"])
	}
}

func TestSpawnUnsupportedRuntimeType(t *testing.T) {
	t.Setenv("RUNTIME_TYPE", "ollama")
	s := setupTestServer(t)

	spawnReq := vmdockerSchema.SpawnRequest{
		Pid:    "pid-1",
		Owner:  "owner-1",
		CuAddr: "cu-1",
	}
	w := performJSONRequest(t, s, http.MethodPost, "/vmm/spawn", spawnReq)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !strings.Contains(res["error"], "runtime type not supported: ollama") {
		t.Fatalf("expected unsupported runtime error, got %q", res["error"])
	}
}

func TestApplyInvalidJSON(t *testing.T) {
	t.Setenv("RUNTIME_TYPE", "test")
	s := setupTestServer(t)

	spawnReq := vmdockerSchema.SpawnRequest{
		Pid:    "pid-1",
		Owner:  "owner-1",
		CuAddr: "cu-1",
	}
	w := performJSONRequest(t, s, http.MethodPost, "/vmm/spawn", spawnReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected spawn status 200, got %d", w.Code)
	}

	w = performRawJSONRequest(s, http.MethodPost, "/vmm/apply", "{")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if res["error"] == "" {
		t.Fatalf("expected bind error message, got empty response: %s", w.Body.String())
	}
}
