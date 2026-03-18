package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cryptowizard0/vmdocker_agent/modulegen"
	"github.com/everFinance/goether"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/permadao/goar"
)

var loadEnvOnce sync.Once

func main() {
	fmt.Println("[module] loading environment from .env")
	loadEnv()

	fmt.Println("[module] generating module artifact")
	artifact, err := modulegen.GenerateModuleArtifact()
	if err != nil {
		fmt.Printf("generate module artifact failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[module] artifact generated: tag_count=%d payload_size=%d bytes\n", len(artifact.Tags), len(artifact.ModuleBytes))

	fmt.Println("[module] initializing sdk")
	client, err := newSDK()
	if err != nil {
		fmt.Printf("init sdk failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[module] saving module bundle to local file")
	itemID, err := client.SaveModule(artifact.ModuleBytes, hymxSchema.Module{
		Base:         hymxSchema.DefaultBaseModule,
		ModuleFormat: modulegen.ModuleFormat,
		Tags:         artifact.Tags,
	})
	if err != nil {
		fmt.Printf("generate and save module failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[module] generate and save module success, id %s\n", itemID)
	fmt.Printf("[module] local bundle file: %s\n", filepath.Join(".", "mod-"+itemID+".json"))
}

func newSDK() (*sdk.SDK, error) {
	url := getEnvWith("VMDOCKER_URL", "http://127.0.0.1:8080")
	prvKey := getEnv("VMDOCKER_PRIVATE_KEY")
	signer, err := goether.NewSigner(prvKey)
	if err != nil {
		return nil, err
	}
	bundler, err := goar.NewBundler(signer)
	if err != nil {
		return nil, err
	}
	return sdk.NewFromBundler(url, bundler), nil
}

func getEnv(key string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	panic(fmt.Sprintf("missing required env %s; set it in .env or your shell environment", key))
}

func getEnvWith(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func loadEnv() {
	loadEnvOnce.Do(func() {
		loadEnvFile(".env")
		loadEnvFile(filepath.Join("cmd", "module", ".env"))
		loadEnvFile(filepath.Join("..", ".env"))
	})
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" || value == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}
