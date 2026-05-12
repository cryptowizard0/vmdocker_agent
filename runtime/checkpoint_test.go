package runtime

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cryptowizard0/vmdocker_agent/harness"
)

func TestEncodeCheckpointEnvelope(t *testing.T) {
	payload, err := encodeCheckpointEnvelope("claude", "claude", harness.Context{
		Workspace:  "/runtime/workspace",
		Home:       "/runtime/.home",
		ContextDir: "/runtime/.vmdocker-agent/context",
		MemoryDir:  "/runtime/.vmdocker-agent/memory",
	}, `{"format":"claudecode.runtime.v1","sessionId":"sess-1"}`)
	if err != nil {
		t.Fatalf("encodeCheckpointEnvelope failed: %v", err)
	}

	var got checkpointEnvelope
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got.Format != checkpointEnvelopeFormatV1 {
		t.Fatalf("format = %q", got.Format)
	}
	if got.Profile != "claude" || got.Backend != "claude" {
		t.Fatalf("profile/backend = %q/%q", got.Profile, got.Backend)
	}
	if got.BackendState == "" {
		t.Fatalf("backend state is empty")
	}
}

func TestDecodeCheckpointEnvelopeRejectsOtherEnvelopeFormats(t *testing.T) {
	_, ok, err := decodeCheckpointEnvelope(`{"format":"vmdocker_agent.runtime.v2"}`)
	if err == nil || ok {
		t.Fatalf("expected unsupported format error, ok=%v err=%v", ok, err)
	}
}

func TestDecodeCheckpointEnvelopeTreatsLegacyStateAsNonEnvelope(t *testing.T) {
	_, ok, err := decodeCheckpointEnvelope(`{"format":"claudecode.runtime.v1","sessionId":"sess-1"}`)
	if err != nil {
		t.Fatalf("decode legacy state returned error: %v", err)
	}
	if ok {
		t.Fatalf("legacy state should not be treated as envelope")
	}
}

func TestDecodeCheckpointEnvelopeRejectsBlankProfileOrBackend(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantErr string
	}{
		{
			name:    "blank profile",
			payload: `{"format":"vmdocker_agent.runtime.v1","profile":"  ","backend":"claude"}`,
			wantErr: "checkpoint envelope profile is required",
		},
		{
			name:    "blank backend",
			payload: `{"format":"vmdocker_agent.runtime.v1","profile":"claude","backend":"  "}`,
			wantErr: "checkpoint envelope backend is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok, err := decodeCheckpointEnvelope(tt.payload)
			if err == nil || ok {
				t.Fatalf("expected validation error, ok=%v err=%v", ok, err)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("err = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestDecodeCheckpointEnvelopeWrapsMalformedEnvelopeError(t *testing.T) {
	_, ok, err := decodeCheckpointEnvelope(`{"format":"vmdocker_agent.runtime.v1","profile":1,"backend":"claude"}`)
	if err == nil || ok {
		t.Fatalf("expected unmarshal error, ok=%v err=%v", ok, err)
	}
	if !strings.Contains(err.Error(), "unmarshal checkpoint envelope failed") {
		t.Fatalf("err = %q", err.Error())
	}
}
