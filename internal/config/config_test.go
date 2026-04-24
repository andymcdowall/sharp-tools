package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestLoadDefaults(t *testing.T) {
	// Create a temporary directory with no config.toml
	tmpDir := t.TempDir()

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Assert returned config matches all default values
	if cfg.Generation.DefaultLanguage != "go" {
		t.Errorf("expected default_language = 'go', got %q", cfg.Generation.DefaultLanguage)
	}
	if cfg.Generation.MaxIterations != 10 {
		t.Errorf("expected max_iterations = 10, got %d", cfg.Generation.MaxIterations)
	}
	if cfg.Execution.Sandbox != "docker" {
		t.Errorf("expected sandbox = 'docker', got %q", cfg.Execution.Sandbox)
	}
	if cfg.Execution.RequireReview != true {
		t.Errorf("expected require_review = true, got %v", cfg.Execution.RequireReview)
	}
	if cfg.Cache.TagOverlapMinScore != 0.6 {
		t.Errorf("expected tag_overlap_min_score = 0.6, got %f", cfg.Cache.TagOverlapMinScore)
	}
	if cfg.Cache.SimilarityThresholdAccept != 0.9 {
		t.Errorf("expected similarity_threshold_accept = 0.9, got %f", cfg.Cache.SimilarityThresholdAccept)
	}
	if cfg.Cache.SimilarityThresholdPrompt != 0.7 {
		t.Errorf("expected similarity_threshold_prompt = 0.7, got %f", cfg.Cache.SimilarityThresholdPrompt)
	}
	if cfg.Cache.AskOnAmbiguousMatch != true {
		t.Errorf("expected ask_on_ambiguous_match = true, got %v", cfg.Cache.AskOnAmbiguousMatch)
	}
	if cfg.Model.IntentParser != "claude-haiku-4-5-20251001" {
		t.Errorf("expected intent_parser = 'claude-haiku-4-5-20251001', got %q", cfg.Model.IntentParser)
	}

	// Assert the file was created
	configFile := filepath.Join(tmpDir, "config.toml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("expected config.toml to be created, but it does not exist")
	}
}

func TestLoadExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a partial config.toml
	partialConfig := `
[generation]
default_language = "python"
max_iterations = 5

[execution]
require_review = false
`
	configFile := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configFile, []byte(partialConfig), 0644); err != nil {
		t.Fatalf("failed to write partial config: %v", err)
	}

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Assert provided values are respected
	if cfg.Generation.DefaultLanguage != "python" {
		t.Errorf("expected default_language = 'python', got %q", cfg.Generation.DefaultLanguage)
	}
	if cfg.Generation.MaxIterations != 5 {
		t.Errorf("expected max_iterations = 5, got %d", cfg.Generation.MaxIterations)
	}
	if cfg.Execution.RequireReview != false {
		t.Errorf("expected require_review = false, got %v", cfg.Execution.RequireReview)
	}

	// Assert unset values fall back to defaults
	if cfg.Execution.Sandbox != "docker" {
		t.Errorf("expected sandbox = 'docker' (default), got %q", cfg.Execution.Sandbox)
	}
	if cfg.Cache.TagOverlapMinScore != 0.6 {
		t.Errorf("expected tag_overlap_min_score = 0.6 (default), got %f", cfg.Cache.TagOverlapMinScore)
	}
}

func TestInvalidLanguage(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a config with invalid default_language
	invalidConfig := `
[generation]
default_language = "ruby"
`
	configFile := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configFile, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	_, err := Load(tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid language, got nil")
	}

	// Assert error message is clear
	if !contains(err.Error(), "invalid default_language") {
		t.Errorf("expected error message to contain 'invalid default_language', got %q", err.Error())
	}
}

func TestConfigFileCreatedOnMiss(t *testing.T) {
	tmpDir := t.TempDir()

	// Call Load on an empty dir
	_, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Assert config.toml now exists on disk
	configFile := filepath.Join(tmpDir, "config.toml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("expected config.toml to be created, but it does not exist")
	}

	// Verify the file is valid TOML
	var loaded map[string]interface{}
	_, err = toml.DecodeFile(configFile, &loaded)
	if err != nil {
		t.Errorf("created config.toml is not valid TOML: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}