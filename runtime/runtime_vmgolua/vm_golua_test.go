package runtimegolua

import (
	"testing"

	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

// go test -run ^TestVmGolua$ -tags "lua53" -v
func TestVmGolua(t *testing.T) {
	env := vmmSchema.Env{}
	vm, err := NewVmGolua(env, "0x84534", "../../vmm/ao/2.0.1", []goarSchema.Tag{})
	if err != nil {
		t.Error("init vm error: ", err)
	}

	// run token.Info
	t.Run("Info handler test", func(t *testing.T) {
		msg := map[string]string{
			"Action":       "Info",
			"From":         "0x123",
			"Module":       "0x84534",
			"Id":           "0x131313",
			"Owner":        "0x123",
			"Block-Height": "100000",
			"Target":       "0x8454",
		}
		_, err := vm.handle("Info", msg)
		if err != nil {
			t.Fatalf("Info handler error: %v", err)
		}
		if err != nil {
			t.Error("Expected non-nil response from Info handler")
		}
	})
}
