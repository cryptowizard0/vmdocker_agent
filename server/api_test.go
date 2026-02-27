package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	vmdockerSchema "github.com/cryptowizard0/vmdocker/vmdocker/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/gin-gonic/gin"
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
