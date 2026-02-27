package runtimeollama

import (
	"context"
	"errors"
	"fmt"

	"github.com/cryptowizard0/vmdocker_agent/common"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/ollama/ollama/api"
	goarSchema "github.com/permadao/goar/schema"
)

var log = common.NewLog("runtimeollama")

type RumtimeOllama struct {
	Client *api.Client
}

func NewRuntimeOllama() (*RumtimeOllama, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		log.Error("create ollama client failed", "err", err)
		return nil, err
	}
	return &RumtimeOllama{
		Client: client,
	}, nil
}
func (r *RumtimeOllama) Apply(from string, meta vmmSchema.Meta, params map[string]string) (vmmSchema.Result, error) {
	if params == nil {
		params = map[string]string{}
	}

	//
	for k, v := range params {
		log.Debug("params", "key", k, "value", v)
		fmt.Println("params ", "key: ", k, ", value: ", v)
	}
	action := params["Action"]
	if action == "" {
		action = meta.Action
	}
	// ! for aos
	if action == "Eval" {
		if params["Data"] == "require('.process')._version" {
			resMsgs := []*vmmSchema.ResMessage{}
			spawnMsgs := []*vmmSchema.ResSpawn{}
			return vmmSchema.Result{
				Messages:    resMsgs,
				Spawns:      spawnMsgs,
				Assignments: nil,
				Output: map[string]string{
					"data":   "2.0.1",
					"prompt": "\u001b[32m\u001b[90m@\u001b[34maos-2.0.1\u001b[90m[Inbox:\u001b[31m\u001b[90m]\u001b[0m\u003e ",
					"test":   "{\n    [1] = \"2.0.1\"\n}",
				},
				Data: "2.0.1",
			}, nil
		}
	}

	// only support chat action
	if action != "Chat" {
		errMsg := errors.New("action not supported: " + action)
		log.Error(errMsg.Error())
		return vmmSchema.Result{}, errMsg
	}

	prompt, exists := params["Prompt"]
	if !exists || prompt == "" {
		errMsg := errors.New("prompt is empty")
		log.Error(errMsg.Error())
		return vmmSchema.Result{}, errMsg
	}

	req := &api.GenerateRequest{
		Model:  "llama3:8b",
		Prompt: prompt,
		Stream: new(bool),
	}

	ctx := context.Background()
	responseText := ""
	respFunc := func(resp api.GenerateResponse) error {
		log.Debug("ollama response", "response", resp)
		fmt.Println("response: ", resp.Response)
		responseText = resp.Response
		return nil
	}
	fmt.Println("prompt: ", prompt)

	err := r.Client.Generate(ctx, req, respFunc)
	if err != nil {
		log.Error("ollama generate failed", "err", err)
		return vmmSchema.Result{}, err
	}
	fmt.Println("responseText: ", responseText)

	outbox := vmmSchema.Result{
		Messages: []*vmmSchema.ResMessage{
			{
				Target:   from,
				Sequence: params["Reference"],
				Data:     responseText,
				Tags: []goarSchema.Tag{
					{Name: "Data-Protocol", Value: "ao"},
					{Name: "Variant", Value: "hymatrix0.1"},
					{Name: "Type", Value: "Message"},
					{Name: "Reference", Value: params["Reference"]},
				},
			},
		},
		Spawns:      []*vmmSchema.ResSpawn{},
		Assignments: nil,
		Output:      responseText,
		Data:        responseText,
	}

	return outbox, nil
}
