package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	if err := os.MkdirAll("bin", 0755); err != nil {
		fmt.Printf("Error creating bin directory: %v\n", err)
		os.Exit(1)
	}

	version := "dev"
	commit := "unknown"
	buildDate := time.Now().UTC().Format(time.RFC3339)

	if out, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output(); err == nil {
		version = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		commit = strings.TrimSpace(string(out))
	}

	ldflags := fmt.Sprintf("-X 'Graft/internal/version.Version=%s' -X 'Graft/internal/version.Commit=%s' -X 'Graft/internal/version.BuildDate=%s'", version, commit, buildDate)

	targets := []struct {
		GOOS   string
		GOARCH string
		Output string
	}{
		{"linux", "amd64", "bin/graft-linux-amd64"},
		{"linux", "arm64", "bin/graft-linux-arm64"},
		{"darwin", "amd64", "bin/graft-darwin-amd64"},
		{"darwin", "arm64", "bin/graft-darwin-arm64"},
		{"windows", "amd64", "bin/graft-windows-amd64.exe"},
	}

	for _, t := range targets {
		fmt.Printf("Building for %s/%s...\n", t.GOOS, t.GOARCH)
		cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", t.Output, "cmd/graft/main.go")
		cmd.Env = append(os.Environ(), "GOOS="+t.GOOS, "GOARCH="+t.GOARCH)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Build failed for %s/%s: %v\n", t.GOOS, t.GOARCH, err)
			os.Exit(1)
		}
	}

	fmt.Println("All builds completed successfully!")
}
