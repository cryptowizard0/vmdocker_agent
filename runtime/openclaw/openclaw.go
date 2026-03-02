package openclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cryptowizard0/vmdocker_agent/common"
	schema "github.com/cryptowizard0/vmdocker_agent/runtime/openclaw/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

const (
	ActionPing          = "Ping"
	ActionQuery         = "Query"
	ActionExecute       = "Execute"
	ActionCreateSession = "CreateSession"
	ActionCloseSession  = "CloseSession"

	DefaultToolCreateSession = "sessions_create"
	DefaultToolSendSession   = "sessions_send"
	DefaultToolCloseSession  = "sessions_delete"
)

var log = common.NewLog("openclaw")

type Openclaw struct {
	mu     sync.RWMutex
	client GatewayClient
	cfg    schema.Config
	state  schema.RuntimeState
}

func LoadConfigFromEnv() schema.Config {
	baseURL := getEnvOrDefault("OPENCLAW_GATEWAY_URL", "http://127.0.0.1:18789")
	timeoutMs, err := strconv.Atoi(getEnvOrDefault("OPENCLAW_TIMEOUT_MS", "30000"))
	if err != nil || timeoutMs <= 0 {
		timeoutMs = 30000
	}
	return schema.Config{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   os.Getenv("OPENCLAW_GATEWAY_TOKEN"),
		Timeout: time.Duration(timeoutMs) * time.Millisecond,
		ActionEndpoints: map[string]schema.Endpoint{
			ActionPing:          {Method: http.MethodGet, Path: getEnvOrDefault("OPENCLAW_ENDPOINT_PING", "/health")},
			ActionQuery:         {Method: http.MethodPost, Path: getEnvOrDefault("OPENCLAW_ENDPOINT_QUERY", "/tools/invoke")},
			ActionExecute:       {Method: http.MethodPost, Path: getEnvOrDefault("OPENCLAW_ENDPOINT_EXECUTE", "/tools/invoke")},
			ActionCreateSession: {Method: http.MethodPost, Path: getEnvOrDefault("OPENCLAW_ENDPOINT_CREATE_SESSION", "/tools/invoke")},
			ActionCloseSession:  {Method: http.MethodPost, Path: getEnvOrDefault("OPENCLAW_ENDPOINT_CLOSE_SESSION", "/tools/invoke")},
		},
	}
}

func New() (*Openclaw, error) {
	cfg := LoadConfigFromEnv()
	client := NewHTTPGatewayClient(cfg)

	initCtx, initCancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer initCancel()
	if err := client.Init(initCtx); err != nil {
		return nil, err
	}

	rt := &Openclaw{
		client: client,
		cfg:    cfg,
		state:  schema.RuntimeState{},
	}

	createCtx, createCancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer createCancel()
	if err := rt.createSession(createCtx); err != nil {
		return nil, err
	}

	return rt, nil
}

func (r *Openclaw) Apply(from string, meta vmmSchema.Meta, params map[string]string) (vmmSchema.Result, error) {
	if params == nil {
		params = map[string]string{}
	}

	action := extractAction(meta, params)
	requestID := extractRequestID(meta, params)
	target := from
	if target == "" {
		target = params["From"]
	}
	if target == "" {
		target = meta.FromProcess
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.Timeout)
	defer cancel()

	var (
		resp         *schema.GatewayResponse
		responseData string
		err          error
	)

	switch action {
	case ActionPing:
		resp, err = r.client.Call(ctx, ActionPing, nil)
		if err != nil {
			return vmmSchema.Result{}, fmt.Errorf("openclaw apply failed: %w", err)
		}
	case ActionCreateSession:
		if err := r.createSession(ctx); err != nil {
			return vmmSchema.Result{}, fmt.Errorf("openclaw apply failed: %w", err)
		}
		sessionID := r.sessionID()
		responseData = sessionID
		resp = &schema.GatewayResponse{
			StatusCode: http.StatusOK,
			Status:     "200 session created",
			Data:       sessionID,
			Body:       sessionID,
			JSON: map[string]interface{}{
				"sessionId": sessionID,
			},
		}
	case ActionCloseSession:
		resp, responseData, err = r.closeSession(ctx)
		if err != nil {
			return vmmSchema.Result{}, fmt.Errorf("openclaw apply failed: %w", err)
		}
	default:
		command := extractCommand(meta, params)
		if command == "" {
			return vmmSchema.Result{}, fmt.Errorf("openclaw apply failed: command is empty")
		}

		sessionID := r.sessionID()
		if sessionID == "" {
			return vmmSchema.Result{}, fmt.Errorf("openclaw apply failed: session is not initialized")
		}

		payload := schema.ToolInvokeRequest{
			Tool: resolveSendTool(action),
		}
		payloadArgs := map[string]interface{}{
			"sessionKey": sessionID,
			"message":    command,
		}
		if timeoutSeconds, ok := extractTimeoutSeconds(params); ok {
			payloadArgs["timeoutSeconds"] = timeoutSeconds
		}
		payload = newToolInvokeRequest(payload.Tool, payloadArgs, sessionID)

		resp, err = r.client.Call(ctx, action, payload)
		if err != nil {
			return vmmSchema.Result{}, fmt.Errorf("openclaw apply failed: %w", err)
		}
	}

	if resp == nil {
		return vmmSchema.Result{}, fmt.Errorf("openclaw apply failed: empty gateway response")
	}

	if responseData == "" {
		responseData = resp.Data
	}
	if responseData == "" {
		responseData = resp.Body
	}

	currentSessionID := r.sessionID()
	tags := []goarSchema.Tag{
		{Name: "Data-Protocol", Value: "ao"},
		{Name: "Variant", Value: "hymatrix0.1"},
		{Name: "Type", Value: "Message"},
		{Name: "Runtime", Value: "openclaw"},
		{Name: "Action", Value: action},
		{Name: "Reference", Value: requestID},
	}

	result := vmmSchema.Result{
		Messages: []*vmmSchema.ResMessage{
			{
				Sequence: requestID,
				Target:   target,
				Data:     responseData,
				Tags:     tags,
			},
		},
		Spawns:      []*vmmSchema.ResSpawn{},
		Assignments: nil,
		Output: map[string]interface{}{
			"runtime":       "openclaw",
			"action":        action,
			"requestId":     requestID,
			"gatewayStatus": resp.Status,
			"statusCode":    resp.StatusCode,
			"sessionId":     currentSessionID,
			"gateway":       resp.JSON,
		},
		Data: responseData,
		Cache: map[string]string{
			"runtime":    "openclaw",
			"action":     action,
			"request_id": requestID,
			"session_id": currentSessionID,
		},
		Error: nil,
	}

	log.Info("openclaw apply success", "action", action, "requestId", requestID, "statusCode", resp.StatusCode)
	return result, nil
}

func (r *Openclaw) createSession(ctx context.Context) error {
	tool := getEnvOrDefault("OPENCLAW_TOOL_CREATE_SESSION", DefaultToolCreateSession)
	title := strings.TrimSpace(os.Getenv("OPENCLAW_SESSION_TITLE"))
	metadata, err := loadSessionMetadataFromEnv()
	if err != nil {
		return err
	}

	args := map[string]interface{}{}
	if title != "" {
		args["title"] = title
	}
	if len(metadata) > 0 {
		args["metadata"] = metadata
	}

	resp, err := r.client.Call(ctx, ActionCreateSession, newToolInvokeRequest(tool, args, ""))
	if err != nil {
		if strings.Contains(err.Error(), "Tool not available") {
			fallbackSessionKey := getEnvOrDefault("OPENCLAW_SESSION_KEY", "main")
			r.mu.Lock()
			r.state.SessionID = fallbackSessionKey
			r.mu.Unlock()
			log.Warn("sessions_create not available, fallback to configured session key", "sessionKey", fallbackSessionKey)
			return nil
		}
		return fmt.Errorf("create openclaw session failed: %w", err)
	}

	sessionID := extractSessionID(resp.JSON)
	if sessionID == "" {
		sessionID = strings.TrimSpace(resp.Data)
	}
	if sessionID == "" {
		return fmt.Errorf("create openclaw session failed: empty session id in response")
	}

	r.mu.Lock()
	r.state.SessionID = sessionID
	r.mu.Unlock()

	log.Info("openclaw session created", "sessionId", sessionID)
	return nil
}

func (r *Openclaw) closeSession(ctx context.Context) (*schema.GatewayResponse, string, error) {
	sessionID := r.sessionID()
	if sessionID == "" {
		return nil, "", fmt.Errorf("openclaw session is empty")
	}

	tool := getEnvOrDefault("OPENCLAW_TOOL_CLOSE_SESSION", DefaultToolCloseSession)
	resp, err := r.client.Call(ctx, ActionCloseSession, newToolInvokeRequest(tool, map[string]interface{}{
		"sessionKey": sessionID,
		"sessionId":  sessionID,
	}, sessionID))
	if err != nil {
		return nil, "", err
	}

	r.mu.Lock()
	r.state.SessionID = ""
	r.mu.Unlock()

	responseData := resp.Data
	if responseData == "" {
		responseData = "closed:" + sessionID
	}
	return resp, responseData, nil
}

func (r *Openclaw) sessionID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.SessionID
}

func extractAction(meta vmmSchema.Meta, params map[string]string) string {
	action := normalizeAction(meta.Action)
	if action != "" {
		return action
	}
	action = normalizeAction(params["action"])
	if action != "" {
		return action
	}
	action = normalizeAction(params["Action"])
	if action != "" {
		return action
	}
	return ActionQuery
}

func extractRequestID(meta vmmSchema.Meta, params map[string]string) string {
	if ref := strings.TrimSpace(params["reference"]); ref != "" {
		return ref
	}
	if ref := strings.TrimSpace(params["Reference"]); ref != "" {
		return ref
	}
	if meta.Sequence > 0 {
		return strconv.FormatInt(meta.Sequence, 10)
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func normalizeAction(action string) string {
	action = strings.TrimSpace(action)
	switch strings.ToLower(action) {
	case "ping":
		return ActionPing
	case "query":
		return ActionQuery
	case "execute":
		return ActionExecute
	case "createsession":
		return ActionCreateSession
	case "closesession":
		return ActionCloseSession
	default:
		return action
	}
}

func extractCommand(meta vmmSchema.Meta, params map[string]string) string {
	for _, key := range []string{"command", "Command", "prompt", "Prompt", "input", "Input", "data", "Data"} {
		if value := strings.TrimSpace(params[key]); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(meta.Data); value != "" {
		return value
	}
	return ""
}

func extractTimeoutSeconds(params map[string]string) (int, bool) {
	for _, key := range []string{"timeoutSeconds", "TimeoutSeconds"} {
		if value := strings.TrimSpace(params[key]); value != "" {
			seconds, err := strconv.Atoi(value)
			if err == nil && seconds >= 0 {
				return seconds, true
			}
		}
	}
	return 0, false
}

func resolveSendTool(action string) string {
	switch action {
	case ActionQuery:
		return getEnvOrDefault("OPENCLAW_TOOL_QUERY", DefaultToolSendSession)
	case ActionExecute:
		return getEnvOrDefault("OPENCLAW_TOOL_EXECUTE", DefaultToolSendSession)
	default:
		return getEnvOrDefault("OPENCLAW_TOOL_SEND_SESSION", DefaultToolSendSession)
	}
}

func newToolInvokeRequest(tool string, args map[string]interface{}, sessionKey string) schema.ToolInvokeRequest {
	req := schema.ToolInvokeRequest{
		Tool:      tool,
		Args:      args,
		Arguments: args,
	}
	if sessionKey != "" {
		req.SessionKey = sessionKey
	}
	return req
}

func loadSessionMetadataFromEnv() (map[string]interface{}, error) {
	raw := strings.TrimSpace(os.Getenv("OPENCLAW_SESSION_METADATA_JSON"))
	if raw == "" {
		return nil, nil
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil, fmt.Errorf("invalid OPENCLAW_SESSION_METADATA_JSON: %w", err)
	}
	return metadata, nil
}

func extractData(v interface{}) string {
	switch vv := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(vv)
	case map[string]interface{}:
		for _, key := range []string{"data", "result", "message", "text", "reply", "output", "content"} {
			if nested, ok := vv[key]; ok {
				if out := extractData(nested); out != "" {
					return out
				}
			}
		}
		for _, nested := range vv {
			switch nested.(type) {
			case map[string]interface{}, []interface{}:
				if out := extractData(nested); out != "" {
					return out
				}
			}
		}
	case []interface{}:
		for _, nested := range vv {
			if out := extractData(nested); out != "" {
				return out
			}
		}
	}
	return ""
}

func extractSessionID(body map[string]interface{}) string {
	if len(body) == 0 {
		return ""
	}

	for _, path := range [][]string{
		{"sessionId"},
		{"sessionID"},
		{"data", "sessionId"},
		{"data", "sessionID"},
		{"data", "session", "id"},
		{"result", "sessionId"},
		{"result", "sessionID"},
		{"result", "session", "id"},
	} {
		if value := lookupStringPath(body, path...); value != "" {
			return value
		}
	}

	return findSessionIDRecursive(body)
}

func lookupStringPath(root map[string]interface{}, path ...string) string {
	var current interface{} = root
	for _, segment := range path {
		nextMap, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		next, ok := nextMap[segment]
		if !ok {
			return ""
		}
		current = next
	}

	text, ok := current.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func findSessionIDRecursive(v interface{}) string {
	switch vv := v.(type) {
	case map[string]interface{}:
		for key, value := range vv {
			if normalizeKey(key) != "sessionid" {
				continue
			}
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
		for _, value := range vv {
			if out := findSessionIDRecursive(value); out != "" {
				return out
			}
		}
	case []interface{}:
		for _, value := range vv {
			if out := findSessionIDRecursive(value); out != "" {
				return out
			}
		}
	}
	return ""
}

func normalizeKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, "-", "")
	return key
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func toJSONString(v interface{}) string {
	buf, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(buf)
}

func getEnvOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
