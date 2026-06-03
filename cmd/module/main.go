package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cryptowizard0/vmdocker_agent/buildmanifest"
	"github.com/cryptowizard0/vmdocker_agent/modulegen"
	"github.com/everFinance/goether"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/permadao/goar"
)

var loadEnvOnce sync.Once

func main() {
	profile := flag.String("profile", "", "build profile name or manifest path")
	flag.Parse()

	fmt.Println("[module] loading environment from .env")
	loadEnv()

	fmt.Println("[module] generating module artifact")
	artifact, err := generateArtifact(*profile)
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
	fmt.Printf("[module] local bundle file: %s\n", filepath.Join(".", "mod", "mod-"+itemID+".json"))
}

func generateArtifact(profile string) (modulegen.ModuleArtifact, error) {
	if strings.TrimSpace(profile) == "" {
		return modulegen.GenerateModuleArtifact()
	}
	profilePath := resolveBuildProfilePath(profile)
	fmt.Printf("[module] loading build profile %s\n", profilePath)
	manifest, err := buildmanifest.Load(profilePath)
	if err != nil {
		return modulegen.ModuleArtifact{}, err
	}
	return modulegen.GenerateModuleArtifactFromManifest(manifest)
}

func resolveBuildProfilePath(profile string) string {
	profile = strings.TrimSpace(profile)
	if strings.HasSuffix(profile, ".toml") || strings.Contains(profile, "/") || strings.Contains(profile, string(filepath.Separator)) {
		return profile
	}
	return filepath.Join("build", "profiles", profile+".toml")
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
