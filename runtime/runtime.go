package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/cryptowizard0/vmdocker_agent/common"
	ollama "github.com/cryptowizard0/vmdocker_agent/runtime/runtime_ollama"
	testrt "github.com/cryptowizard0/vmdocker_agent/runtime/runtime_testrt"
	golua "github.com/cryptowizard0/vmdocker_agent/runtime/runtime_vmgolua"
	"github.com/cryptowizard0/vmdocker_agent/runtime/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

var log = common.NewLog("runtime")

const (
	RuntimeTypeGolua  = "golua"
	RuntimeTypeOLlama = "ollama"
	RuntimeTypeTest   = "test"
)

type Runtime struct {
	vm schema.IRuntime
}

// func New(pid, owner, cuAddr, aoDir string, data []byte, tags []goarSchema.Tag) (*Runtime, error) {
func New(env vmmSchema.Env, nodeAddr, aoDir string, tags []goarSchema.Tag) (*Runtime, error) {
	var vm schema.IRuntime
	var err error

	runtimeType := RuntimeTypeGolua
	if envType := os.Getenv("RUNTIME_TYPE"); envType != "" {
		runtimeType = envType
	}
	fmt.Println("runtime type: ", runtimeType)

	switch runtimeType {
	case RuntimeTypeGolua:
		vm, err = golua.NewVmGolua(env, nodeAddr, aoDir, tags)
	case RuntimeTypeOLlama:
		vm, err = ollama.NewRuntimeOllama()
	case RuntimeTypeTest:
		vm, err = testrt.NewRuntimeTest()
	default:
		return nil, errors.New("runtime type not supported: " + runtimeType)
	}

	if err != nil {
		return nil, err
	}

	return &Runtime{
		vm: vm,
	}, nil
}

func (r *Runtime) Apply(from string, meta vmmSchema.Meta, params map[string]string) (string, error) {
	response, err := r.vm.Apply(from, meta, params)
	if err != nil {
		return "", errors.New(fmt.Sprintf("runtime apply failed: %s", err.Error()))
	}
	outboxJson, err := json.Marshal(response)
	if err != nil {
		log.Error("marshal outbox failed", "err", err)
		return "", err
	}
	return string(outboxJson), nil
}
