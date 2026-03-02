package openclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	schema "github.com/cryptowizard0/vmdocker_agent/runtime/openclaw/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func (r *Openclaw) SetupOnSpawn(ctx context.Context, params map[string]string) error {
	if _, err := r.CreateSession(ctx); err != nil {
		return err
	}

	if model := extractModelName(vmmSchema.Meta{}, params); model != "" {
		if _, err := r.ConfigureModel(ctx, model); err != nil {
			return err
		}
	}

	if len(buildTelegramConfigPatch(params)) > 0 {
		if _, err := r.ConfigureTelegram(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (r *Openclaw) ConfigureModel(ctx context.Context, model string) (*schema.GatewayResponse, error) {
	if model == "" {
		return nil, fmt.Errorf("model is empty")
	}
	sessionID := r.sessionID()
	if sessionID == "" {
		return nil, fmt.Errorf("session is not initialized")
	}
	tool := resolveSetModelTool()
	args := map[string]interface{}{
		"sessionKey": sessionID,
		"model":      model,
	}
	payload := newToolInvokeRequest(tool, args, sessionID)
	resp, err := r.client.Call(ctx, ActionConfigureModel, payload)
	if err != nil {
		return nil, fmt.Errorf("configure model failed: %w", err)
	}
	return resp, nil
}

func (r *Openclaw) ConfigureTelegram(ctx context.Context, params map[string]string) (*schema.GatewayResponse, error) {
	patch := buildTelegramConfigPatch(params)
	if len(patch) == 0 {
		return nil, fmt.Errorf("empty telegram patch")
	}
	rawPatch, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshal telegram patch failed: %w", err)
	}
	args := map[string]interface{}{
		"action": "config.patch",
		"raw":    string(rawPatch),
	}
	payload := newToolInvokeRequest(resolveGatewayTool(), args, "")
	resp, err := r.client.Call(ctx, ActionConfigureTelegram, payload)
	if err != nil {
		return nil, fmt.Errorf("configure telegram failed: %w", err)
	}
	return resp, nil
}

func (r *Openclaw) CreateSession(ctx context.Context) (*schema.GatewayResponse, error) {
	tool := getEnvOrDefault("OPENCLAW_TOOL_CREATE_SESSION", DefaultToolCreateSession)
	title := strings.TrimSpace(os.Getenv("OPENCLAW_SESSION_TITLE"))
	metadata, err := loadSessionMetadataFromEnv()
	if err != nil {
		return nil, err
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
			return &schema.GatewayResponse{
				StatusCode: http.StatusOK,
				Status:     "200 session created (fallback)",
				Data:       fallbackSessionKey,
				Body:       fallbackSessionKey,
				JSON: map[string]interface{}{
					"sessionId": fallbackSessionKey,
				},
			}, nil
		}
		return nil, fmt.Errorf("create openclaw session failed: %w", err)
	}

	sessionID := extractSessionID(resp.JSON)
	if sessionID == "" {
		sessionID = strings.TrimSpace(resp.Data)
	}
	if sessionID == "" {
		return nil, fmt.Errorf("create openclaw session failed: empty session id in response")
	}

	r.mu.Lock()
	r.state.SessionID = sessionID
	r.mu.Unlock()

	log.Info("openclaw session created", "sessionId", sessionID)
	return resp, nil
}
