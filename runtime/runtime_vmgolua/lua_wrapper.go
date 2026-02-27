// lua_wrapper.go provides wrapper functions for interacting with the Lua VM.
//
// The package uses the golua/lua library for Lua integration and provides
// a higher-level interface for AO-specific operations.
package runtimegolua

import (
	"fmt"

	"github.com/aarzilli/golua/lua"
)

// lua_unsafe_pcall defines a Lua function that wraps unsafe_pcall
// to provide a safer interface for function calls.
const lua_unsafe_pcall = `
function pcall(f, arg1, ...)
	return unsafe_pcall(f, arg1, ...)
end
`

const lua_get_outbox = `
local json = require('json')
local ao = require('ao')
local json_outbox = json.encode(ao.outbox)
return json_outbox
`

// SetPackagePath sets the Lua package path.
func SetPackagePath(L *lua.State, path string) error {
	return L.DoString(fmt.Sprintf("package.path = package.path .. ';%s'", path))
}

// SetPackageCPath sets the Lua package C path.
func SetPackageCPath(L *lua.State, path string) error {
	return L.DoString(fmt.Sprintf("package.cpath = package.cpath .. ';%s'", path))
}

// GetOutbox retrieves the current outbox content from the Lua state.
// It executes a Lua script that serializes the ao.outbox to JSON.
//
// Parameters:
//   - L: the Lua state
//
// Returns:
//   - string: JSON string containing the outbox content
//   - error: if script execution or JSON serialization fails
func GetOutbox(L *lua.State) (string, error) {
	err := L.DoString(lua_get_outbox)
	if err != nil {
		return "", err
	}
	return L.ToString(-1), nil
}

// ProcessHandle executes the process.handle function in the Lua VM with the given message and environment.
//
// Parameters:
//   - jsonMsg: JSON string containing the message to be processed
//   - jsonEnv: JSON string containing the environment configuration
//
// Returns:
//   - string: JSON string containing the result
//   - error: if any error occurs during Lua code execution
func ProcessHandle(L *lua.State, jsonMsg, jsonEnv string) (string, error) {
	// execute lua code
	// call process.handle(msg, env) with lua string
	// then get the result from ao.outbox after execution
	payload := `
		require('ao')
		local process = require('process')
		local json = require('json')
		local json_msg = [=[%s]=]
		local json_env = [=[%s]=]
		local table_msg = json.decode(json_msg)
		local table_evn = json.decode(json_env)
		local resp = process.handle(table_msg, table_evn)
		local json_outbox = json.encode(resp)
		return json_outbox
	`
	payload = fmt.Sprintf(payload, string(jsonMsg), string(jsonEnv))
	err := L.DoString(payload)
	if err != nil {
		return "", err
	}

	// get result from ao.outbox
	jsonResult := L.ToString(-1)

	return jsonResult, nil
}

func GetTable(L *lua.State, name string) {
	L.GetGlobal(name)
	if !L.IsTable(-1) {
		fmt.Println("Error: ", name, " is not a table, type: ", L.Type(-1))
		return
	}

	L.PushNil()
	for L.Next(-2) != 0 {
		fmt.Printf("%s - %s\n", L.ToString(-2), L.ToString(-1))
		L.Pop(1)
	}
}

func GetString(L *lua.State, name string) string {
	L.GetGlobal(name)
	if !L.IsString(-1) {
		fmt.Println("Error: ", name, " is not a string")
		return ""
	}
	return L.ToString(-1)
}

func GetType(L *lua.State, name string) lua.LuaValType {
	L.GetGlobal(name)
	return L.Type(-1)
}
