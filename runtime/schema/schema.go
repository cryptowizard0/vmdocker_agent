package schema

import (
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

type IRuntime interface {
	Apply(from string, meta vmmSchema.Meta, params map[string]string) (vmmSchema.Result, error)
}
