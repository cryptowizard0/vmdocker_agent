package openclaw

import (
	"bytes"
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
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

const (
	ActionPing          = "Ping"
	ActionQuery         = "Query"
	ActionExecute       = "Execute"
	ActionCreateSession = "CreateSession"
	ActionCloseSession  = "CloseSession"
)

var log = common.NewLog("openclaw")

type Endpoint struct {
	Method string
	Path   string
}

type Config struct {
	BaseURL         string
	Token           string
	Timeout         time.Duration
	ActionEndpoints map[string]Endpoint
}

type RuntimeState struct {
	SessionID string
}

type GatewayResponse struct {
	StatusCode int
	Status     string
	Data       string
	Body       string
	JSON       map[string]interface{}
}

type GatewayClient interface {
	Init(ctx context.Context) error
	Call(ctx context.Context, action string, payload map[string]string) (*GatewayResponse, error)
	Close(ctx context.Context) error
}

type Runtime struct {
	mu     sync.RWMutex
	client GatewayClient
	cfg    Config
	state  RuntimeState
}

type HTTPGatewayClient struct {
	cfg    Config
	client *http.Client
}

func LoadConfigFromEnv() Config {
	baseURL := getEnvOrDefault("OPENCLAW_GATEWAY_URL", "http://127.0.0.1:18789")
	timeoutMs, err := strconv.Atoi(getEnvOrDefault("OPENCLAW_TIMEOUT_MS", "30000"))
	if err != nil || timeoutMs <= 0 {
		timeoutMs = 30000
	}
	return Config{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   os.Getenv("OPENCLAW_GATEWAY_TOKEN"),
		Timeout: time.Duration(timeoutMs) * time.Millisecond,
		ActionEndpoints: map[string]Endpoint{
			ActionPing:          {Method: http.MethodGet, Path: getEnvOrDefault("OPENCLAW_ENDPOINT_PING", "/health")},
			ActionQuery:         {Method: http.MethodPost, Path: getEnvOrDefault("OPENCLAW_ENDPOINT_QUERY", "/v1/tools/invoke")},
			ActionExecute:       {Method: http.MethodPost, Path: getEnvOrDefault("OPENCLAW_ENDPOINT_EXECUTE", "/v1/tools/invoke")},
			ActionCreateSession: {Method: http.MethodPost, Path: getEnvOrDefault("OPENCLAW_ENDPOINT_CREATE_SESSION", "/v1/tools/invoke")},
			ActionCloseSession:  {Method: http.MethodPost, Path: getEnvOrDefault("OPENCLAW_ENDPOINT_CLOSE_SESSION", "/v1/tools/invoke")},
		},
	}
}

func NewRuntime() (*Runtime, error) {
	cfg := LoadConfigFromEnv()
	client := NewHTTPGatewayClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	if err := client.Init(ctx); err != nil {
		return nil, err
	}
	return &Runtime{
		client: client,
		cfg:    cfg,
		state:  RuntimeState{},
	}, nil
}

func NewHTTPGatewayClient(cfg Config) *HTTPGatewayClient {
	return &HTTPGatewayClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *HTTPGatewayClient) Init(ctx context.Context) error {
	resp, err := c.Call(ctx, ActionPing, nil)
	if err != nil {
		return fmt.Errorf("openclaw gateway init failed: %w", err)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("openclaw gateway unhealthy status: %d", resp.StatusCode)
	}
	return nil
}

func (c *HTTPGatewayClient) Call(ctx context.Context, action string, payload map[string]string) (*GatewayResponse, error) {
	action = normalizeAction(action)
	ep, ok := c.cfg.ActionEndpoints[action]
	if !ok {
		return nil, fmt.Errorf("openclaw action not supported: %s", action)
	}

	url := c.cfg.BaseURL + normalizePath(ep.Path)
	var bodyReader *bytes.Reader
	if ep.Method == http.MethodPost {
		if payload == nil {
			payload = map[string]string{}
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal openclaw payload failed: %w", err)
		}
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, ep.Method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create openclaw request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call openclaw gateway failed: %w", err)
	}
	defer res.Body.Close()

	var response GatewayResponse
	response.StatusCode = res.StatusCode
	response.Status = res.Status

	var bodyMap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&bodyMap); err != nil {
		return nil, fmt.Errorf("decode openclaw gateway response failed: %w", err)
	}
	response.JSON = bodyMap
	response.Body = toJSONString(bodyMap)
	response.Data = extractData(bodyMap)

	if res.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("openclaw gateway error status=%d body=%s", res.StatusCode, response.Body)
	}

	return &response, nil
}

func (c *HTTPGatewayClient) Close(_ context.Context) error {
	return nil
}

func (r *Runtime) Apply(from string, meta vmmSchema.Meta, params map[string]string) (vmmSchema.Result, error) {
	if params == nil {
		params = map[string]string{}
	}
	action := extractAction(meta, params)
	requestID := extractRequestID(meta, params)
	target := from
	if target == "" {
		target = params["From"]
	}

	payload := map[string]string{
		"requestId": requestID,
		"action":    action,
		"from":      from,
		"pid":       meta.Pid,
		"itemId":    meta.ItemId,
		"sequence":  strconv.FormatInt(meta.Sequence, 10),
		"data":      meta.Data,
	}
	for k, v := range params {
		payload[k] = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.Timeout)
	defer cancel()

	resp, err := r.client.Call(ctx, action, payload)
	if err != nil {
		return vmmSchema.Result{}, fmt.Errorf("openclaw apply failed: %w", err)
	}

	responseData := resp.Data
	if responseData == "" {
		responseData = resp.Body
	}

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
			"gateway":       resp.JSON,
		},
		Data: responseData,
		Cache: map[string]string{
			"runtime":    "openclaw",
			"action":     action,
			"request_id": requestID,
		},
		Error: nil,
	}

	log.Info("openclaw apply success", "action", action, "requestId", requestID, "statusCode", resp.StatusCode)
	return result, nil
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

func extractData(m map[string]interface{}) string {
	for _, k := range []string{"data", "result", "message"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
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
