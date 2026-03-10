package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cryptowizard0/vmdocker_agent/utils"
)

func main() {
	if len(os.Args) < 2 {
		runPrepare(os.Args[1:])
		return
	}

	switch os.Args[1] {
	case "prepare":
		runPrepare(os.Args[2:])
	default:
		runPrepare(os.Args[1:])
	}
}

func runPrepare(args []string) {
	fs := flag.NewFlagSet("prepare", flag.ExitOnError)
	shellOutput := fs.Bool("shell", false, "print shell assignments")
	_ = fs.Parse(args)

	paths, err := utils.PrepareOpenclawRuntime(os.Getenv, os.UserHomeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare openclaw runtime failed: %v\n", err)
		os.Exit(1)
	}

	if *shellOutput {
		fmt.Printf("OPENCLAW_STATE_DIR=%s\n", shellQuote(paths.StateDir))
		fmt.Printf("OPENCLAW_CONFIG_PATH=%s\n", shellQuote(paths.ConfigPath))
		fmt.Printf("OPENCLAW_GATEWAY_LOG_PATH=%s\n", shellQuote(paths.GatewayLogPath))
		return
	}

	fmt.Println(paths.ConfigPath)
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
