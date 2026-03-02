package runtime

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cryptowizard0/vmdocker_agent/common"
	"github.com/cryptowizard0/vmdocker_agent/runtime/openclaw"
	"github.com/cryptowizard0/vmdocker_agent/runtime/schema"
	"github.com/cryptowizard0/vmdocker_agent/runtime/testrt"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

var log = common.NewLog("runtime")

const (
	RuntimeTypeTest     = "test"
	RuntimeTypeOpenclaw = "openclaw"
)

type Runtime struct {
	vm schema.IRuntime
}

func New(env vmmSchema.Env, nodeAddr, aoDir string, tags []goarSchema.Tag, spawnParams map[string]string) (*Runtime, error) {
	var vm schema.IRuntime
	var err error

	runtimeType := RuntimeTypeTest
	if envType := os.Getenv("RUNTIME_TYPE"); envType != "" {
		runtimeType = envType
	}
	log.Info("runtime type selected", "type", runtimeType)

	switch runtimeType {
	case RuntimeTypeTest:
		vm, err = testrt.NewRuntimeTest()
	case RuntimeTypeOpenclaw:
		vm, err = openclaw.NewWithParams(spawnParams)
	default:
		return nil, fmt.Errorf("runtime type not supported: %s", runtimeType)
	}

	if err != nil {
		return nil, err
	}

	return &Runtime{vm: vm}, nil
}

func (r *Runtime) Apply(from string, meta vmmSchema.Meta, params map[string]string) (string, error) {
	response, err := r.vm.Apply(from, meta, params)
	if err != nil {
		return "", fmt.Errorf("runtime apply failed: %w", err)
	}
	outboxJson, err := json.Marshal(response)
	if err != nil {
		log.Error("marshal outbox failed", "err", err)
		return "", err
	}
	return string(outboxJson), nil
}
