package hermes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cryptowizard0/vmdocker_agent/common"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/xingj404-lab/agent-hub/hermesAgent"
)

const checkpointFormatV1 = "hermes.runtime.v1"

var log = common.NewLog("hermes_agent")
var (
	hermesMu       sync.Mutex
	hermes         *hermesAgent.HermesAgent
	hermesStarting bool
	hermesLastErr  string
)

type Config struct {
	Cwd         string
	BotToken    string
	LLMProvider string
	LLMBaseURL  string
	LLMApiKey   string
	LLMModel    string
	Running     bool // true if hermes was running at last checkpoint

	AccessServerURL string
	BrowserAPIKey   string
}

type checkpointState struct {
	Format          string `json:"format"`
	SessionID       string `json:"sessionId,omitempty"`
	Cwd             string `json:"cwd,omitempty"`
	BotToken        string `json:"botToken,omitempty"`
	LLMProvider     string `json:"llmProvider,omitempty"`
	LLMBaseURL      string `json:"llmBaseURL,omitempty"`
	LLMApiKey       string `json:"llmApiKey,omitempty"`
	LLMModel        string `json:"llmModel,omitempty"`
	Running         bool   `json:"running,omitempty"`
	AccessServerURL string `json:"accessServerUrl,omitempty"`
	BrowserAPIKey   string `json:"browserApiKey,omitempty"`
}

type Runtime struct {
	mu     sync.RWMutex
	config Config
	state  checkpointState
}

func NewWithParams(spawnParams map[string]string) (*Runtime, error) {
	cfg, err := loadConfig(spawnParams)
	if err != nil {
		return nil, err
	}

	return &Runtime{
		config: cfg,
		state: checkpointState{
			Format:    checkpointFormatV1,
			SessionID: newSessionID(),
			Cwd:       cfg.Cwd,
		},
	}, nil
}

func NewRestored(state string, spawnParams map[string]string) (*Runtime, error) {
	cfg, err := loadConfig(spawnParams)
	if err != nil {
		return nil, err
	}

	rt := &Runtime{
		config: cfg,
	}
	if err := rt.Restore(state); err != nil {
		return nil, err
	}

	// 如果 checkpoint 记录 hermes 正在运行，则自动拉起
	if rt.config.Running && rt.config.BotToken != "" {
		if err := startHermes(rt.config); err != nil {
			return nil, err
		}
	}
	return rt, nil
}

func loadConfig(spawnParams map[string]string) (Config, error) {
	cwd := strings.TrimSpace(os.Getenv("VMDOCKER_AGENT_WORKSPACE"))
	if cwd == "" {
		cwd = strings.TrimSpace(os.Getenv("VMDOCKER_RUNTIME_WORKSPACE"))
	}
	if cwd == "" {
		cwd = strings.TrimSpace(os.Getenv("OPENCLAW_AGENT_WORKSPACE"))
	}
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Config{}, fmt.Errorf("resolve runtime workspace failed: %w", err)
		}
		cwd = wd
	}
	cwd, err := filepath.Abs(cwd)
	if err != nil {
		return Config{}, fmt.Errorf("resolve runtime workspace %s failed: %w", cwd, err)
	}

	return Config{
		Cwd: cwd,
	}, nil
}

// resolveHomeDir returns the home directory for skill installation.
// Prefers VMDOCKER_RUNTIME_HOME if set, falls back to $HOME.
func resolveHomeDir() string {
	homeDir := strings.TrimSpace(os.Getenv("VMDOCKER_RUNTIME_HOME"))
	if homeDir == "" {
		homeDir = strings.TrimSpace(os.Getenv("HOME"))
	}
	homeDir, _ = filepath.Abs(homeDir)
	return homeDir
}

func startHermes(cfg Config) error {
	if cfg.BotToken == "" {
		return fmt.Errorf("bot_token is required")
	}
	if cfg.LLMModel == "" {
		return fmt.Errorf("llmModel is required")
	}

	agent := hermesAgent.New(hermesAgent.Config{
		LLMProvider:      cfg.LLMProvider,
		LLMModel:         cfg.LLMModel,
		LLMBaseURL:       cfg.LLMBaseURL,
		LLMAPIKey:        cfg.LLMApiKey,
		TelegramBotToken: cfg.BotToken,
		AccessServerURL:  cfg.AccessServerURL,
		BrowserAPIKey:    cfg.BrowserAPIKey,
	})
	if err := agent.Run(); err != nil {
		agent.Close()
		return err
	}

	hermesMu.Lock()
	hermes = agent
	hermesLastErr = ""
	hermesMu.Unlock()
	return nil
}

func (r *Runtime) Apply(from string, meta vmmSchema.Meta, params map[string]string) (res vmmSchema.Result, err error) {
	switch meta.Action {
	case "start":
		hermesMu.Lock()
		if hermes != nil {
			hermesMu.Unlock()
			res.Error = fmt.Errorf("hermes already running")
			return res, nil
		}
		if hermesStarting {
			hermesMu.Unlock()
			res.Error = fmt.Errorf("hermes is starting")
			return res, nil
		}
		hermesStarting = true
		hermesLastErr = ""
		hermesMu.Unlock()

		provider := firstNonEmptyParam(params, "LLM_PROVIDER", "llmProvider", "provider")
		if provider == "" {
			provider = "custom"
		}
		baseURL := firstNonEmptyParam(params, "LLM_BASE_URL", "LLM_BASEURL", "llmBaseURL", "baseURL")
		if baseURL == "" {
			markHermesStartFinished("LLM_BASE_URL is required")
			res.Error = fmt.Errorf("LLM_BASE_URL is required")
			return res, nil
		}
		apiKey := firstNonEmptyParam(params, "LLM_API_KEY", "llmApiKey", "apiKey")
		if apiKey == "" {
			markHermesStartFinished("LLM_API_KEY is required")
			res.Error = fmt.Errorf("LLM_API_KEY is required")
			return res, nil
		}
		model := firstNonEmptyParam(params, "LLM_MODEL", "llmModel", "model")
		if model == "" {
			markHermesStartFinished("LLM_MODEL is required")
			res.Error = fmt.Errorf("LLM_MODEL is required")
			return res, nil
		}
		botToken := firstNonEmptyParam(params, "Bot_Token", "BOT_TOKEN", "botToken", "telegramBotToken")
		if botToken == "" {
			markHermesStartFinished("bot_token is required")
			res.Error = fmt.Errorf("bot_token is required")
			return res, nil
		}

		accessUrl := firstNonEmptyParam(params, "ACCESS_SERVER_URL", "accessServerURL", "accessUrl")
		if accessUrl == "" {
			markHermesStartFinished("ACCESS_SERVER_URL is required")
			res.Error = fmt.Errorf("ACCESS_SERVER_URL is required")
			return res, nil
		}
		browserApiKey := firstNonEmptyParam(params, "BROWSER_API_KEY", "browserAPIKey", "browserApiKey")
		if browserApiKey == "" {
			markHermesStartFinished("BROWSER_API_KEY is required")
			res.Error = fmt.Errorf("BROWSER_API_KEY is required")
			return res, nil
		}

		r.mu.Lock()
		r.config.LLMProvider = provider
		r.config.LLMBaseURL = baseURL
		r.config.LLMApiKey = apiKey
		r.config.LLMModel = model
		r.config.BotToken = botToken
		r.config.Running = true
		r.config.AccessServerURL = accessUrl
		r.config.BrowserAPIKey = browserApiKey
		cfg := r.config
		r.mu.Unlock()

		go r.startHermesInBackground(cfg)
		res.Data = "hermes start requested"
		return res, nil
	case "stop":
		hermesMu.Lock()
		if hermesStarting {
			hermesMu.Unlock()
			res.Error = fmt.Errorf("hermes is starting")
			return res, nil
		}
		agent := hermes
		if agent == nil {
			hermesMu.Unlock()
			res.Error = fmt.Errorf("hermes already stopped")
			return res, nil
		}
		hermes = nil
		hermesLastErr = ""
		hermesMu.Unlock()

		agent.Close()
		r.mu.Lock()
		r.config.Running = false
		r.mu.Unlock()
		return res, nil
	case "status":
		res.Data = hermesStatus()
		return res, nil
	}
	return vmmSchema.Result{}, fmt.Errorf("hermes apply is not implemented")
}

func (r *Runtime) startHermesInBackground(cfg Config) {
	log.Info("hermes async start begin")
	err := startHermes(cfg)

	hermesMu.Lock()
	hermesStarting = false
	if err != nil {
		hermesLastErr = err.Error()
	}
	hermesMu.Unlock()

	if err != nil {
		log.Error("hermes async start failed", "err", err)
		r.mu.Lock()
		r.config.Running = false
		r.mu.Unlock()
		return
	}
	log.Info("hermes async start complete")
}

func markHermesStartFinished(lastErr string) {
	hermesMu.Lock()
	hermesStarting = false
	hermesLastErr = lastErr
	hermesMu.Unlock()
}

func hermesStatus() string {
	hermesMu.Lock()
	defer hermesMu.Unlock()

	switch {
	case hermes != nil:
		return "running"
	case hermesStarting:
		return "starting"
	case hermesLastErr != "":
		return "start_failed: " + hermesLastErr
	default:
		return "stopped"
	}
}

func (r *Runtime) Checkpoint() (string, error) {
	r.mu.RLock()
	cfg := r.config
	r.mu.RUnlock()

	state := checkpointState{
		Format:          checkpointFormatV1,
		SessionID:       "",
		Cwd:             cfg.Cwd,
		BotToken:        cfg.BotToken,
		LLMProvider:     cfg.LLMProvider,
		LLMBaseURL:      cfg.LLMBaseURL,
		LLMApiKey:       cfg.LLMApiKey,
		LLMModel:        cfg.LLMModel,
		Running:         cfg.Running,
		AccessServerURL: cfg.AccessServerURL,
		BrowserAPIKey:   cfg.BrowserAPIKey,
	}

	payload, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("marshal hermes checkpoint failed: %w", err)
	}
	return string(payload), nil
}

func (r *Runtime) Restore(data string) error {
	if strings.TrimSpace(data) == "" {
		return fmt.Errorf("hermes checkpoint state is empty")
	}

	var state checkpointState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return fmt.Errorf("decode hermes checkpoint failed: %w", err)
	}
	if state.Format != "" && state.Format != checkpointFormatV1 {
		return fmt.Errorf("unsupported hermes checkpoint format: %s", state.Format)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.state = state

	if cwd := strings.TrimSpace(state.Cwd); cwd != "" {
		r.config.Cwd = cwd
	}
	r.config.BotToken = strings.TrimSpace(state.BotToken)
	r.config.LLMProvider = strings.TrimSpace(state.LLMProvider)
	r.config.LLMBaseURL = strings.TrimSpace(state.LLMBaseURL)
	r.config.LLMApiKey = strings.TrimSpace(state.LLMApiKey)
	r.config.LLMModel = strings.TrimSpace(state.LLMModel)
	r.config.Running = state.Running
	r.config.AccessServerURL = strings.TrimSpace(state.AccessServerURL)
	r.config.BrowserAPIKey = strings.TrimSpace(state.BrowserAPIKey)

	return nil
}

func newSessionID() string {
	return fmt.Sprintf("hermes-%d", time.Now().UnixNano())
}

func firstNonEmptyParam(params map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(params[key]); value != "" {
			return value
		}
	}
	return ""
}
