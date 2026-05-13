package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cryptowizard0/vmdocker_agent/buildmanifest"
)

func main() {
	manifestPath := flag.String("profile", "build/profiles/claude.toml", "build profile manifest path")
	flag.Parse()

	manifest, err := buildmanifest.Load(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load build profile failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("build profile: %s\n", manifest.Name)
	fmt.Printf("runtime profile: %s\n", manifest.RuntimeProfile)
	fmt.Printf("image: %s\n", manifest.ImageName)
	fmt.Printf("start command: %s\n", manifest.StartCommand)
}
