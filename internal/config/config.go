package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration for Sharp Tools
type Config struct {
	Generation GenerationConfig `toml:"generation"`
	Execution  ExecutionConfig  `toml:"execution"`
	Cache      CacheConfig      `toml:"cache"`
	Model      ModelConfig      `toml:"model"`
}

// GenerationConfig holds configuration for tool generation
type GenerationConfig struct {
	DefaultLanguage string `toml:"default_language"` // "go" | "python" | "typescript"
	MaxIterations   int    `toml:"max_iterations"`   // vX — sidecar manages its own loop in v0
}

// ExecutionConfig holds configuration for tool execution
type ExecutionConfig struct {
	Sandbox       string `toml:"sandbox"`        // "docker" | "none"
	RequireReview bool   `toml:"require_review"`
}

// CacheConfig holds configuration for cache lookup
type CacheConfig struct {
	TagOverlapMinScore        float64 `toml:"tag_overlap_min_score"`         // vX
	SimilarityThresholdAccept float64 `toml:"similarity_threshold_accept"`  // vX
	SimilarityThresholdPrompt float64 `toml:"similarity_threshold_prompt"`  // vX
	AskOnAmbiguousMatch       bool    `toml:"ask_on_ambiguous_match"`       // vX
}

// ModelConfig holds configuration for LLM models
type ModelConfig struct {
	IntentParser string `toml:"intent_parser"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		Generation: GenerationConfig{
			DefaultLanguage: "go",
			MaxIterations:   10,
		},
		Execution: ExecutionConfig{
			Sandbox:       "docker",
			RequireReview: true,
		},
		Cache: CacheConfig{
			TagOverlapMinScore:        0.6,
			SimilarityThresholdAccept: 0.9,
			SimilarityThresholdPrompt: 0.7,
			AskOnAmbiguousMatch:       true,
		},
		Model: ModelConfig{
			IntentParser: "claude-haiku-4-5-20251001",
		},
	}
}

// Load reads configuration from the config file in the given directory.
// If no config file exists, it creates one with default values.
// Returns the configuration and any error encountered.
func Load(configDir string) (*Config, error) {
	configFile := filepath.Join(configDir, "config.toml")
	
	// Check if config file exists
	_, err := os.Stat(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file doesn't exist, create defaults and write
			cfg := DefaultConfig()
			if err := writeConfigFile(configFile, cfg); err != nil {
				return nil, fmt.Errorf("failed to write default config: %w", err)
			}
			return cfg, nil
		}
		// Other error - return it
		return nil, fmt.Errorf("failed to stat config file: %w", err)
	}
	
	// Config file exists, start with defaults
	cfg := DefaultConfig()
	
	// Decode directly into the struct (toml tags handle the mapping)
	if _, err := toml.DecodeFile(configFile, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	// Validate DefaultLanguage
	if err := validateLanguage(cfg.Generation.DefaultLanguage); err != nil {
		return nil, err
	}
	
	return cfg, nil
}

// writeConfigFile writes the configuration to the given file path
func writeConfigFile(path string, cfg *Config) error {
	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Create the file with default config
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()
	
	// Encode and write the default config
	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	
	return nil
}

// validateLanguage checks if the given language is valid
func validateLanguage(language string) error {
	validLanguages := map[string]bool{
		"go":         true,
		"python":     true,
		"typescript": true,
	}
	
	if !validLanguages[language] {
		return fmt.Errorf("invalid default_language: %q (must be one of: go, python, typescript)", language)
	}
	
	return nil
}