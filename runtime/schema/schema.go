package schema

import (
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

const (
	ModuleFormatGolua  = "golua"
	ModuleFormatOLlama = "ollama"
)

type IRuntime interface {
	Apply(from string, meta vmmSchema.Meta, params map[string]string) (vmmSchema.Result, error)
}

type AoProcess struct {
	Owner string           `json:"Owner"`
	Id    string           `json:"Id"`
	Tags  []goarSchema.Tag `json:"Tags"`
}

type AoModule struct {
	Owner string           `json:"Owner"`
	Id    string           `json:"Id"`
	Tags  []goarSchema.Tag `json:"Tags"`
}

type AoEnv struct {
	Module  AoModule  `json:"Module"`
	Process AoProcess `json:"Process"`
}
