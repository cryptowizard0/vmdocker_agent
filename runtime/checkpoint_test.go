package runtime

import (
	"encoding/json"
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
