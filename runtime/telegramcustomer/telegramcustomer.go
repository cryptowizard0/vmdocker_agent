package telegramcustomer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cryptowizard0/vmdocker_agent/common"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	customer "github.com/xingj404-lab/agent-hub/tgcustomer"
)

const checkpointFormatV1 = "telegramcustomer.runtime.v1"

var log = common.NewLog("tgcustomer")
var csTg *customer.Customer

// RunByHymx is the entry point for launching the customer service.
// Exposed as a var for test mocking.
var RunByHymx = customer.RunByHymx

// Config holds all persistent settings for the telegram customer runtime.
// These values are saved to checkpoint and restored on restart.
type Config struct {
	Cwd         string
	BotToken    string
	LLMProvider string
	LLMBaseURL  string
	LLMApiKey   string
	LLMModel    string
	Running     bool // true if customer was running at last checkpoint
}

type checkpointState struct {
	Format      string `json:"format"`
	SessionID   string `json:"sessionId,omitempty"`
	Cwd         string `json:"cwd,omitempty"`
	BotToken    string `json:"botToken,omitempty"`
	LLMProvider string `json:"llmProvider,omitempty"`
	LLMBaseURL  string `json:"llmBaseURL,omitempty"`
	LLMApiKey   string `json:"llmApiKey,omitempty"`
	LLMModel    string `json:"llmModel,omitempty"`
	Running     bool   `json:"running,omitempty"`
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

	// 如果 checkpoint 记录 customer 正在运行，则自动拉起
	if rt.config.Running && rt.config.BotToken != "" {
		if err := startCustomer(rt.config); err != nil {
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

// startCustomer configures hermes LLM and launches the customer service.
func startCustomer(cfg Config) error {
	if cfg.BotToken == "" {
		return fmt.Errorf("bot_token is required")
	}
	if cfg.LLMModel == "" {
		return fmt.Errorf("LLM_MODEL is required")
	}

	if err := configureHermesModel(cfg.LLMProvider, cfg.LLMBaseURL, cfg.LLMApiKey, cfg.LLMModel); err != nil {
		return err
	}

	var err error
	csTg, err = RunByHymx(cfg.Cwd, resolveHomeDir(), cfg.BotToken)
	return err
}

// configureHermesModel sets hermes LLM provider config via CLI.
func configureHermesModel(provider, baseURL, apiKey, model string) error {
	cmds := []struct {
		key   string
		value string
	}{
		{"model.provider", provider},
		{"model.base_url", baseURL},
		{"model.api_key", apiKey},
		{"model.default", model},
	}

	for _, cmd := range cmds {
		if cmd.value == "" {
			continue
		}
		out, err := exec.Command("hermes", "config", "set", cmd.key, cmd.value).CombinedOutput()
		if err != nil {
			return fmt.Errorf("hermes config set %s failed: %s: %w", cmd.key, string(out), err)
		}
		log.Info("hermes config set", "key", cmd.key)
	}
	return nil
}

// hermesAllowedCmds is a whitelist of hermes CLI subcommands that are
// non-interactive and return results immediately. Commands not listed here
// are rejected to prevent hanging processes (e.g. chat, dashboard) or
// interactive flows (e.g. login, setup).
var hermesAllowedCmds = map[string]bool{
	// config
	"config:":         true,
	"config:show":     true,
	"config:set":      true,
	"config:path":     true,
	"config:env-path": true,
	"config:check":    true,
	"config:migrate":  true,

	// status / version / dump / doctor
	"status":  true,
	"version": true,
	"dump":    true,
	"doctor":  true,

	// sessions
	"sessions:list":   true,
	"sessions:export": true,
	"sessions:delete": true,
	"sessions:prune":  true,
	"sessions:stats":  true,
	"sessions:rename": true,

	// logs (only without -f)
	"logs": true,

	// insights
	"insights": true,

	// cron
	"cron:list":   true,
	"cron:create": true,
	"cron:add":    true,
	"cron:edit":   true,
	"cron:pause":  true,
	"cron:resume": true,
	"cron:run":    true,
	"cron:remove": true,
	"cron:rm":     true,
	"cron:delete": true,
	"cron:status": true,
	"cron:tick":   true,

	// hooks
	"hooks:list":   true,
	"hooks:ls":     true,
	"hooks:test":   true,
	"hooks:revoke": true,
	"hooks:remove": true,
	"hooks:rm":     true,
	"hooks:doctor": true,

	// skills
	"skills:search":    true,
	"skills:install":   true,
	"skills:inspect":   true,
	"skills:list":      true,
	"skills:check":     true,
	"skills:update":    true,
	"skills:audit":     true,
	"skills:uninstall": true,
	"skills:reset":     true,
	"skills:publish":   true,
	"skills:snapshot":  true,
	"skills:tap":       true,

	// profile
	"profile:list":    true,
	"profile:use":     true,
	"profile:create":  true,
	"profile:delete":  true,
	"profile:show":    true,
	"profile:alias":   true,
	"profile:rename":  true,
	"profile:export":  true,
	"profile:import":  true,
	"profile:install": true,
	"profile:update":  true,
	"profile:info":    true,

	// auth
	"auth:list":   true,
	"auth:remove": true,
	"auth:reset":  true,
	"auth:status": true,
	"auth:logout": true,

	// mcp
	"mcp:list": true,
	"mcp:ls":   true,
	"mcp:test": true,

	// webhook
	"webhook:list":   true,
	"webhook:ls":     true,
	"webhook:remove": true,
	"webhook:rm":     true,
	"webhook:test":   true,

	// gateway
	"gateway:status": true,
	"gateway:list":   true,

	// fallback
	"fallback:list": true,
	"fallback:ls":   true,

	// update (--check only, bare update would be destructive)
	"update:check": true,

	// tools
	"tools:list":    true,
	"tools:summary": true,

	// plugins
	"plugins:list": true,
	"plugins:ls":   true,

	// checkpoints
	"checkpoints:status":       true,
	"checkpoints:list":         true,
	"checkpoints:prune":        true,
	"checkpoints:clear":        true,
	"checkpoints:clear-legacy": true,

	// backup / debug
	"backup":      true,
	"debug:share": true,

	// lsp
	"lsp:status": true,
	"lsp:list":   true,
	"lsp:which":  true,

	// memory
	"memory:status": true,

	// computer-use
	"computer-use:status": true,

	// dashboard
	"dashboard:status": true,

	// kanban (non-streaming subcommands)
	"kanban:init":        true,
	"kanban:boards":      true,
	"kanban:create":      true,
	"kanban:list":        true,
	"kanban:ls":          true,
	"kanban:show":        true,
	"kanban:assign":      true,
	"kanban:reclaim":     true,
	"kanban:reassign":    true,
	"kanban:diagnostics": true,
	"kanban:diag":        true,
	"kanban:link":        true,
	"kanban:unlink":      true,
	"kanban:comment":     true,
	"kanban:complete":    true,
	"kanban:edit":        true,
	"kanban:block":       true,
	"kanban:unblock":     true,
	"kanban:archive":     true,
	"kanban:stats":       true,
	"kanban:log":         true,
	"kanban:runs":        true,
	"kanban:heartbeat":   true,
	"kanban:assignees":   true,
	"kanban:context":     true,
	"kanban:specify":     true,
	"kanban:gc":          true,

	// curator
	"curator:status":        true,
	"curator:run":           true,
	"curator:pause":         true,
	"curator:resume":        true,
	"curator:pin":           true,
	"curator:unpin":         true,
	"curator:list-archived": true,
	"curator:archive":       true,
	"curator:prune":         true,
	"curator:backup":        true,
	"curator:rollback":      true,
}

// validateHermesCmd checks if a command line is allowed by the whitelist.
// It requires the command to start with "hermes", then looks up the
// top-level subcommand + optional second-level subcommand in the allowed set.
// Returns the actual exec args (everything after "hermes") on success, or
// a rejection error for interactive/long-running commands.
func validateHermesCmd(cmd string) ([]string, error) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("call is required")
	}
	if parts[0] != "hermes" {
		return nil, fmt.Errorf("only hermes CLI commands are allowed, got: %s", parts[0])
	}
	if len(parts) < 2 {
		// bare "hermes" enters interactive chat — reject
		return nil, fmt.Errorf("hermes requires a subcommand (e.g. hermes status, hermes config show)")
	}

	top := parts[1]
	sub := ""
	if len(parts) > 2 {
		sub = parts[2]
	}

	// Try top:sub first, then fall back to top: (prefix match)
	if hermesAllowedCmds[top+":"+sub] {
		return parts[1:], nil
	}
	if hermesAllowedCmds[top+":"] {
		return parts[1:], nil
	}
	// No subcommand at all — try bare top-level
	if hermesAllowedCmds[top] {
		return parts[1:], nil
	}

	return nil, fmt.Errorf("hermes command %s is not allowed (interactive or long-running commands are blocked)", cmd)
}

func (r *Runtime) Apply(from string, meta vmmSchema.Meta, params map[string]string) (res vmmSchema.Result, err error) {
	switch meta.Action {
	case "start":
		if csTg != nil {
			res.Error = fmt.Errorf("a customer is already running, send stop first")
			return res, nil
		}

		provider := params["LLM_PROVIDER"]
		if provider == "" {
			provider = "custom"
		}
		baseURL := params["LLM_BASE_URL"]
		if baseURL == "" {
			res.Error = fmt.Errorf("LLM_BASE_URL is required")
			return res, nil
		}
		apiKey := params["LLM_API_KEY"]
		if apiKey == "" {
			res.Error = fmt.Errorf("LLM_API_KEY is required")
			return res, nil
		}
		model := params["LLM_MODEL"]
		if model == "" {
			res.Error = fmt.Errorf("LLM_MODEL is required")
			return res, nil
		}
		botToken := params["Bot_Token"]
		if botToken == "" {
			res.Error = fmt.Errorf("bot_token is required")
			return res, nil
		}

		// 保存到 config 以便 checkpoint/restore
		r.mu.Lock()
		r.config.LLMProvider = provider
		r.config.LLMBaseURL = baseURL
		r.config.LLMApiKey = apiKey
		r.config.LLMModel = model
		r.config.BotToken = botToken
		r.config.Running = true
		r.mu.Unlock()

		if err := startCustomer(r.config); err != nil {
			res.Error = err
			return res, nil
		}
		return res, nil
	case "stop":
		if csTg == nil {
			res.Error = fmt.Errorf("no customer is running")
			return res, nil
		}
		csTg.Close()
		csTg = nil

		// 记录停止状态，checkpoint 后 restore 时不再自动启动
		r.mu.Lock()
		r.config.Running = false
		r.mu.Unlock()
		return res, nil
	case "call_hermes":
		cmd := params["call"]
		if cmd == "" {
			res.Error = fmt.Errorf("call is required")
			return res, nil
		}

		args, err := validateHermesCmd(cmd)
		if err != nil {
			res.Error = err
			return res, nil
		}

		out, err := exec.Command("hermes", args...).CombinedOutput()
		if err != nil {
			res.Error = fmt.Errorf("call hermes failed: %s: %w", string(out), err)
			return res, nil
		}
		res.Data = string(out)
		return res, nil

	}
	return vmmSchema.Result{}, fmt.Errorf("telegramcustomer apply is not implemented")
}

func (r *Runtime) Checkpoint() (string, error) {
	r.mu.RLock()
	cfg := r.config
	r.mu.RUnlock()

	state := checkpointState{
		Format:      checkpointFormatV1,
		Cwd:         cfg.Cwd,
		BotToken:    cfg.BotToken,
		LLMProvider: cfg.LLMProvider,
		LLMBaseURL:  cfg.LLMBaseURL,
		LLMApiKey:   cfg.LLMApiKey,
		LLMModel:    cfg.LLMModel,
		Running:     cfg.Running,
	}

	payload, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("marshal telegramcustomer checkpoint failed: %w", err)
	}
	return string(payload), nil
}

func (r *Runtime) Restore(data string) error {
	if strings.TrimSpace(data) == "" {
		return fmt.Errorf("telegramcustomer checkpoint state is empty")
	}

	var state checkpointState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return fmt.Errorf("decode telegramcustomer checkpoint failed: %w", err)
	}
	if state.Format != "" && state.Format != checkpointFormatV1 {
		return fmt.Errorf("unsupported telegramcustomer checkpoint format: %s", state.Format)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.state = state

	r.config.Cwd = strings.TrimSpace(state.Cwd)
	r.config.BotToken = strings.TrimSpace(state.BotToken)
	r.config.LLMProvider = strings.TrimSpace(state.LLMProvider)
	r.config.LLMBaseURL = strings.TrimSpace(state.LLMBaseURL)
	r.config.LLMApiKey = strings.TrimSpace(state.LLMApiKey)
	r.config.LLMModel = strings.TrimSpace(state.LLMModel)
	r.config.Running = state.Running

	return nil
}

func newSessionID() string {
	return fmt.Sprintf("telegramcustomer-%d", time.Now().UnixNano())
}
