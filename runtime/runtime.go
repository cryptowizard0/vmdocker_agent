package runtime

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cryptowizard0/vmdocker_agent/common"
	"github.com/cryptowizard0/vmdocker_agent/harness"
	"github.com/cryptowizard0/vmdocker_agent/runtime/backend"
	claudeBackend "github.com/cryptowizard0/vmdocker_agent/runtime/backend/claude"
	legacyBackend "github.com/cryptowizard0/vmdocker_agent/runtime/backend/legacy"
	"github.com/cryptowizard0/vmdocker_agent/runtime/openclaw"
	"github.com/cryptowizard0/vmdocker_agent/runtime/profile"
	"github.com/cryptowizard0/vmdocker_agent/runtime/telegramcustomer"
	"github.com/cryptowizard0/vmdocker_agent/runtime/testrt"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

var log = common.NewLog("runtime")

const (
	RuntimeTypeTest             = "test"
	RuntimeTypeOpenclaw         = "openclaw"
	RuntimeTypeClaude           = "claude"
	RuntimeTypeTelegramCustomer = "telegramcustomer"
)

var harnessEnvKeys = []string{
	"VMDOCKER_AGENT_ASSET_ROOT",
	"VMDOCKER_AGENT_SKILLS_DIR",
	"VMDOCKER_AGENT_ROLE_PATH",
	"VMDOCKER_AGENT_CONTEXT_DIR",
	"VMDOCKER_AGENT_MEMORY_DIR",
}

type Runtime struct {
	backend     backend.Backend
	profileName string
	backendName string
	harnessCtx  harness.Context
}

func New(env vmmSchema.Env, nodeAddr, aoDir string, tags []goarSchema.Tag, spawnParams map[string]string) (*Runtime, error) {
	return newRuntime(env, nodeAddr, aoDir, tags, spawnParams, "", false)
}

func NewRestored(env vmmSchema.Env, nodeAddr, aoDir string, tags []goarSchema.Tag, state string) (*Runtime, error) {
	return newRuntime(env, nodeAddr, aoDir, tags, tagsToParams(tags), state, true)
}

func newRuntime(env vmmSchema.Env, nodeAddr, aoDir string, tags []goarSchema.Tag, spawnParams map[string]string, state string, restore bool) (*Runtime, error) {
	selectedProfile := profile.ResolveSelector(os.Getenv)
	profileDir := profile.ResolveProfileDir(os.Getenv)
	envelope, hasEnvelope, err := decodeCheckpointEnvelope(state)
	if err != nil {
		return nil, err
	}
	if restore && hasEnvelope {
		selectedProfile = envelope.Profile
		state = envelope.BackendState
	}

	prof, err := profile.Load(profileDir, selectedProfile)
	if err != nil {
		if os.Getenv(profile.EnvAgentProfile) == "" && os.Getenv(profile.EnvRuntimeType) != "" && selectedProfile == os.Getenv(profile.EnvRuntimeType) {
			return nil, fmt.Errorf("runtime type not supported: %s", selectedProfile)
		}
		return nil, err
	}
	harnessCtx, err := harness.Init(prof, os.Getenv)
	if err != nil {
		return nil, err
	}
	if err := applyHarnessEnv(harnessCtx.Env); err != nil {
		return nil, err
	}

	cfg := backend.Config{
		Harness:     harnessCtx,
		Env:         env,
		NodeAddr:    nodeAddr,
		AODir:       aoDir,
		Tags:        tags,
		SpawnParams: spawnParams,
		State:       state,
		Restore:     restore,
	}
	vm, err := newBackend(prof.Backend, cfg)
	if err != nil {
		return nil, err
	}

	log.Info("runtime profile selected", "profile", prof.Name, "backend", prof.Backend)
	return &Runtime{
		backend:     vm,
		profileName: prof.Name,
		backendName: prof.Backend,
		harnessCtx:  harnessCtx,
	}, nil
}

func newBackend(name string, cfg backend.Config) (backend.Backend, error) {
	switch name {
	case RuntimeTypeClaude:
		return claudeBackend.New(cfg)
	case RuntimeTypeOpenclaw, "openclaw-legacy":
		if cfg.Restore {
			rt, err := openclaw.NewRestored(cfg.State)
			if err != nil {
				return nil, err
			}
			return legacyBackend.New(rt), nil
		}
		rt, err := openclaw.NewWithParams(cfg.SpawnParams)
		if err != nil {
			return nil, err
		}
		return legacyBackend.New(rt), nil
	case RuntimeTypeTelegramCustomer, "telegramcustomer-legacy":
		if cfg.Restore {
			rt, err := telegramcustomer.NewRestored(cfg.State, cfg.SpawnParams)
			if err != nil {
				return nil, err
			}
			return legacyBackend.New(rt), nil
		}
		rt, err := telegramcustomer.NewWithParams(cfg.SpawnParams)
		if err != nil {
			return nil, err
		}
		return legacyBackend.New(rt), nil
	case RuntimeTypeTest:
		rt, err := testrt.NewRuntimeTest()
		if err != nil {
			return nil, err
		}
		return legacyBackend.New(rt), nil
	default:
		return nil, fmt.Errorf("runtime backend not supported: %s", name)
	}
}

func applyHarnessEnv(env map[string]string) error {
	for _, key := range harnessEnvKeys {
		value, ok := env[key]
		if !ok {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("apply harness env %s failed: %w", key, err)
		}
	}
	return nil
}

func tagsToParams(tags []goarSchema.Tag) map[string]string {
	params := make(map[string]string, len(tags))
	for _, tag := range tags {
		params[tag.Name] = tag.Value
	}
	return params
}

func (r *Runtime) Apply(from string, meta vmmSchema.Meta, params map[string]string) (string, error) {
	response, err := r.backend.Apply(backend.Request{From: from, Meta: meta, Params: params})
	if err != nil {
		return "", fmt.Errorf("runtime apply failed: %w", err)
	}
	outboxJson, err := json.Marshal(response.Result)
	if err != nil {
		log.Error("marshal outbox failed", "err", err)
		return "", err
	}
	return string(outboxJson), nil
}

func (r *Runtime) Checkpoint() (string, error) {
	if r == nil || r.backend == nil {
		return "", fmt.Errorf("runtime is nil")
	}
	backendState, err := r.backend.Checkpoint()
	if err != nil {
		return "", err
	}
	return encodeCheckpointEnvelope(r.profileName, r.backendName, r.harnessCtx, backendState)
}

func (r *Runtime) Restore(data string) error {
	if r == nil || r.backend == nil {
		return fmt.Errorf("runtime is nil")
	}
	envelope, ok, err := decodeCheckpointEnvelope(data)
	if err != nil {
		return err
	}
	if ok {
		return r.backend.Restore(envelope.BackendState)
	}
	return r.backend.Restore(data)
}
