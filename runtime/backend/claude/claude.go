package claude

import (
	"github.com/cryptowizard0/vmdocker_agent/runtime/backend"
	"github.com/cryptowizard0/vmdocker_agent/runtime/claudecode"
)

type Backend struct {
	runtime *claudecode.Runtime
}

func New(cfg backend.Config) (*Backend, error) {
	var rt *claudecode.Runtime
	var err error
	if cfg.Restore {
		rt, err = claudecode.NewRestored(cfg.State, cfg.SpawnParams)
	} else {
		rt, err = claudecode.NewWithParams(cfg.SpawnParams)
	}
	if err != nil {
		return nil, err
	}
	return &Backend{runtime: rt}, nil
}

func (b *Backend) Apply(req backend.Request) (backend.Response, error) {
	result, err := b.runtime.Apply(req.From, req.Meta, req.Params)
	if err != nil {
		return backend.Response{}, err
	}
	return backend.Response{Result: result}, nil
}

func (b *Backend) Checkpoint() (string, error) {
	return b.runtime.Checkpoint()
}

func (b *Backend) Restore(data string) error {
	return b.runtime.Restore(data)
}
