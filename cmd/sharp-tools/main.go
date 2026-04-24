package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sharp-tools/sharp-tools/internal/config"
)

// App holds the application state
type App struct {
	Config     *config.Config
	APIKey     string
	ConfigDir  string
}

func main() {
	// Get API key from environment
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: ANTHROPIC_API_KEY is not set")
		os.Exit(1)
	}

	// Determine config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to determine home directory: %v\n", err)
		os.Exit(1)
	}

	configDir := filepath.Join(homeDir, ".sharp-tools")

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create config directory: %v\n", err)
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create app instance
	app := &App{
		Config:    cfg,
		APIKey:    apiKey,
		ConfigDir: configDir,
	}

	// TODO: Wire CLI commands here
	_ = app
}