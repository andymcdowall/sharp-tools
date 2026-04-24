package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration for Sharp Tools
type Config struct {
	Generation GenerationConfig
	Execution  ExecutionConfig
	Cache      CacheConfig
	Model      ModelConfig
}

// GenerationConfig holds configuration for tool generation
type GenerationConfig struct {
	DefaultLanguage string // "go" | "python" | "typescript"
	MaxIterations   int    // vX — sidecar manages its own loop in v0
}

// ExecutionConfig holds configuration for tool execution
type ExecutionConfig struct {
	Sandbox       string // "docker" | "none"
	RequireReview bool
}

// CacheConfig holds configuration for cache lookup
type CacheConfig struct {
	TagOverlapMinScore         float64
	SimilarityThresholdAccept  float64 // vX
	SimilarityThresholdPrompt  float64 // vX
	AskOnAmbiguousMatch        bool    // vX
}

// ModelConfig holds configuration for LLM models
type ModelConfig struct {
	IntentParser string
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
	if os.IsNotExist(err) {
		// Config file doesn't exist, create defaults and write
		cfg := DefaultConfig()
		if err := writeConfigFile(configFile, cfg); err != nil {
			return nil, fmt.Errorf("failed to write default config: %w", err)
		}
		return cfg, nil
	}
	
	// Config file exists, read and parse it
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	
	// Decode into a map first
	var rawConfig map[string]interface{}
	_, err = toml.Decode(string(data), &rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	// Start with defaults
	cfg := DefaultConfig()
	
	// Apply generation config
	if gen, ok := rawConfig["generation"].(map[string]interface{}); ok {
		if v, ok := gen["default_language"].(string); ok {
			cfg.Generation.DefaultLanguage = v
		}
		if v, ok := gen["max_iterations"].(int64); ok {
			cfg.Generation.MaxIterations = int(v)
		}
	}
	
	// Apply execution config
	if exec, ok := rawConfig["execution"].(map[string]interface{}); ok {
		if v, ok := exec["sandbox"].(string); ok {
			cfg.Execution.Sandbox = v
		}
		if v, ok := exec["require_review"].(bool); ok {
			cfg.Execution.RequireReview = v
		}
	}
	
	// Apply cache config
	if cache, ok := rawConfig["cache"].(map[string]interface{}); ok {
		if v, ok := cache["tag_overlap_min_score"].(float64); ok {
			cfg.Cache.TagOverlapMinScore = v
		}
		if v, ok := cache["similarity_threshold_accept"].(float64); ok {
			cfg.Cache.SimilarityThresholdAccept = v
		}
		if v, ok := cache["similarity_threshold_prompt"].(float64); ok {
			cfg.Cache.SimilarityThresholdPrompt = v
		}
		if v, ok := cache["ask_on_ambiguous_match"].(bool); ok {
			cfg.Cache.AskOnAmbiguousMatch = v
		}
	}
	
	// Apply model config
	if model, ok := rawConfig["model"].(map[string]interface{}); ok {
		if v, ok := model["intent_parser"].(string); ok {
			cfg.Model.IntentParser = v
		}
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