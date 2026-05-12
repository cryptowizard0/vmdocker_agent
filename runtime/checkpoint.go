package runtime

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cryptowizard0/vmdocker_agent/harness"
)

const checkpointEnvelopeFormatV1 = "vmdocker_agent.runtime.v1"

type checkpointEnvelope struct {
	Format       string            `json:"format"`
	Profile      string            `json:"profile"`
	Backend      string            `json:"backend"`
	Harness      checkpointHarness `json:"harness"`
	BackendState string            `json:"backendState"`
}

type checkpointHarness struct {
	Workspace string `json:"workspace"`
	Home      string `json:"home"`
	Context   string `json:"context"`
	Memory    string `json:"memory"`
}

func encodeCheckpointEnvelope(profileName, backendName string, harnessCtx harness.Context, backendState string) (string, error) {
	payload, err := json.Marshal(checkpointEnvelope{
		Format:  checkpointEnvelopeFormatV1,
		Profile: profileName,
		Backend: backendName,
		Harness: checkpointHarness{
			Workspace: harnessCtx.Workspace,
			Home:      harnessCtx.Home,
			Context:   harnessCtx.ContextDir,
			Memory:    harnessCtx.MemoryDir,
		},
		BackendState: backendState,
	})
	if err != nil {
		return "", fmt.Errorf("marshal checkpoint envelope failed: %w", err)
	}
	return string(payload), nil
}

func decodeCheckpointEnvelope(data string) (checkpointEnvelope, bool, error) {
	if strings.TrimSpace(data) == "" {
		return checkpointEnvelope{}, false, nil
	}

	var header struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal([]byte(data), &header); err != nil || header.Format == "" {
		return checkpointEnvelope{}, false, nil
	}

	if header.Format != checkpointEnvelopeFormatV1 {
		if strings.HasPrefix(header.Format, "vmdocker_agent.") {
			return checkpointEnvelope{}, false, fmt.Errorf("unsupported checkpoint envelope format: %s", header.Format)
		}
		return checkpointEnvelope{}, false, nil
	}

	var envelope checkpointEnvelope
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return checkpointEnvelope{}, false, fmt.Errorf("unmarshal checkpoint envelope failed: %w", err)
	}
	if strings.TrimSpace(envelope.Profile) == "" {
		return checkpointEnvelope{}, false, fmt.Errorf("checkpoint envelope profile is required")
	}
	if strings.TrimSpace(envelope.Backend) == "" {
		return checkpointEnvelope{}, false, fmt.Errorf("checkpoint envelope backend is required")
	}
	return envelope, true, nil
}
