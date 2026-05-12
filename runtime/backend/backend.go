package backend

import (
	"github.com/cryptowizard0/vmdocker_agent/harness"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type Backend interface {
	Apply(Request) (Response, error)
	Checkpoint() (string, error)
	Restore(string) error
}

type Config struct {
	Harness     harness.Context
	Env         vmmSchema.Env
	NodeAddr    string
	AODir       string
	Tags        []goarSchema.Tag
	SpawnParams map[string]string
	State       string
	Restore     bool
}

type Request struct {
	From   string
	Meta   vmmSchema.Meta
	Params map[string]string
}

type Response struct {
	Result vmmSchema.Result
}
