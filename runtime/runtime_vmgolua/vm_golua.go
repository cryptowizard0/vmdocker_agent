package runtimegolua

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/aarzilli/golua/lua"
	"github.com/cryptowizard0/vmdocker_agent/common"
	"github.com/cryptowizard0/vmdocker_agent/runtime/schema"
	"github.com/cryptowizard0/vmdocker_agent/utils"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

var log = common.NewLog("vmm")

type LuaPath struct {
	Path        string
	CPath       string
	ProcessPath string
}

type VmGolua struct {
	L       *lua.State
	Env     *schema.AoEnv
	LuaPath *LuaPath
	mLock   sync.Mutex
}

func (v *VmGolua) Restore(bytes []byte) error {
	// TODO implement me
	panic("implement me")
}

func (v *VmGolua) CheckPoint(nonce uint64) ([]byte, error) {
	// TODO implement me
	panic("implement me")
}

// NewVmGolua creates and initializes a new Lua VM instance for AO process execution.
//
// Returns:
//   - *VmGolua: A pointer to the initialized VM instance.
//   - error: Any error encountered during initialization, or nil if successful.
//
// func NewVmGolua(pid, owner, cuAddr, aoDir string, data []byte, tags []goarSchema.Tag) (*VmGolua, error) {
func NewVmGolua(env vmmSchema.Env, nodeAddr, aoDir string, tags []goarSchema.Tag) (*VmGolua, error) {
	// Initialize new Lua state with standard libraries
	L := lua.NewState()
	L.OpenLibs()

	// TODO: In future iterations, these paths should be get from data
	// Currently using local filesystem paths for development
	// aoDir := "../../vmm/ao/2.0.1"
	luaPath := LuaPath{
		Path:        fmt.Sprintf("%s/?.lua", aoDir),              // Path for Lua modules
		CPath:       fmt.Sprintf("%s/?.so;%s/?.a", aoDir, aoDir), // Path for C modules
		ProcessPath: fmt.Sprintf("%s/process.lua", aoDir),        // Main process script path
	}

	var err error
	processID := env.Meta.Pid
	if processID == "" {
		processID = env.Meta.ItemId
	}
	aoEnv := schema.AoEnv{
		Module: schema.AoModule{
			Owner: env.Meta.AccId,
			Id:    "0x456",
			Tags:  env.Module.Tags,
		},
		Process: schema.AoProcess{
			Owner: env.Meta.AccId,
			Id:    processID,
			Tags:  env.Process.Tags,
		},
	}

	// Initialize VM instance with all required components
	v := &VmGolua{
		L:       L,        // Lua state
		LuaPath: &luaPath, // Lua path configuration
		Env:     &aoEnv,   // AO environment
	}

	// Initialize the Lua environment with required modules and configurations
	err = v.initEvn()
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (v *VmGolua) Apply(from string, meta vmmSchema.Meta, params map[string]string) (vmmSchema.Result, error) {
	if params == nil {
		params = map[string]string{}
	}

	params["Id"] = meta.ItemId
	params["Action"] = meta.Action
	params["From"] = from
	params["Owner"] = meta.AccId
	params["Target"] = meta.Pid
	params["Timestamp"] = fmt.Sprintf("%d", meta.Timestamp)
	params["Pushed-For"] = meta.PushedFor

	result, err := v.handle(meta.Action, params)
	if err != nil {
		return vmmSchema.Result{}, err
	}
	log.Debug("==> process handle result", "result", utils.PrettyJSON(result))
	fmt.Printf("\n==> Result:\n%s\n", utils.PrettyJSON(result))

	// err = utils.HandleOutbox(result, v.Env)
	// if err != nil {
	// 	return vmmSchema.Result{}, err
	// }
	outbox := vmmSchema.Result{}
	result = strings.Replace(result, `"Cache":[]`, `"Cache":{}`, 1)
	err = json.Unmarshal([]byte(result), &outbox)
	if err != nil {
		return vmmSchema.Result{}, err
	}
	// if outbox.Error != "" {
	// 	return vmmSchema.Result{}, errors.New(outbox.Error)
	// }

	return outbox, nil
}

// initEvn initializes the Lua environment for the VM instance.
// It performs the following setup steps:
// 1. Sets the Lua package path for module loading
// 2. Sets the Lua C module path for native extensions
// 3. Replaces standard 'pcall' with 'unsafe_pcall' for better error handling
// 4. Loads and initializes the AO process from the specified path
//
// Returns:
//   - error: if any initialization step fails, with a descriptive error message
//     indicating which step failed and why
func (v *VmGolua) initEvn() error {
	v.mLock.Lock()
	defer v.mLock.Unlock()

	// set lua path
	err := SetPackagePath(v.L, v.LuaPath.Path)
	if err != nil {
		msg := fmt.Sprintf("set lua path error: %s", err)
		return errors.New(msg)

	}
	// set lua cpath
	err = SetPackageCPath(v.L, v.LuaPath.CPath)
	if err != nil {
		msg := fmt.Sprintf("set lua cpath error: %s", err)
		return errors.New(msg)
	}
	// replace 'pcall' with 'unsafe_pcall'
	err = v.L.DoString(lua_unsafe_pcall)
	if err != nil {
		return errors.New(fmt.Sprintf("init lua env error: %s", err))
	}

	// create ao process
	// todo: maybe load Module from arweave
	err = v.L.DoFile(v.LuaPath.ProcessPath)
	if err != nil {
		return errors.New(fmt.Sprintf("load ao process failed: %s", err))
	}

	return nil
}

// handle processes an action with given parameters by executing it in the Lua VM.
// It converts the input parameters into a message format that the Lua process can understand,
// executes the message handler, and returns the resulting outbox content.
//
// Parameters:
//   - action: the action to be performed (currently unused in implementation)
//   - params: a map of key-value pairs representing the message parameters
//
// Returns:
//   - result: the result of the action execution
//   - outbox: the serialized ao.outbox content from the Lua execution
//   - err: if any error occurs during message handling or Lua execution
func (v *VmGolua) handle(action string, params map[string]string) (result string, err error) {
	v.mLock.Lock()
	defer v.mLock.Unlock()

	fmt.Println("===> handle params: ", params)

	result = ""
	msg := map[string]interface{}{}
	tags := []goarSchema.Tag{}
	for k, v := range params {
		if k != "Timestamp" {
			tag := goarSchema.Tag{
				Name:  k,
				Value: v,
			}
			tags = append(tags, tag)
		}
		msg[k] = v
	}
	msg["Tags"] = tags

	// convert timestamp to int64
	if timestampStr, ok := msg["Timestamp"].(string); ok && timestampStr != "" {
		if timestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
			msg["Timestamp"] = timestamp
		}
	}

	// env may be changed, so we need to marshal it every time
	jsonEnv, err := json.Marshal(v.Env)
	if err != nil {
		err = errors.New(fmt.Sprintf("marshal ao env failed: %s", err))
		return
	}

	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		return
	}

	fmt.Printf("\n==> apply msg:\n%s\n", utils.PrettyJSON(string(jsonMsg)))
	fmt.Printf("\n==> apply env:\n%s\n", utils.PrettyJSON(string(jsonEnv)))

	result, err = ProcessHandle(v.L, string(jsonMsg), string(jsonEnv))
	if err != nil {
		return
	}

	// get result from ao.outbox
	// outbox, err = GetOutbox(v.L)
	return
}
