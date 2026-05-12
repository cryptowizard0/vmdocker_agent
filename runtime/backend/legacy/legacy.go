package legacy

import (
	"github.com/cryptowizard0/vmdocker_agent/runtime/backend"
	"github.com/cryptowizard0/vmdocker_agent/runtime/schema"
)

type Backend struct {
	runtime schema.IRuntime
}

func New(rt schema.IRuntime) *Backend {
	return &Backend{runtime: rt}
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
