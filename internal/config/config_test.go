package config

import (
	"os"
	"path/filepath"
	"strings"
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
	return strings.Contains(s, substr)
}

func TestRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// First load - creates default config
	cfg1, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("first Load() returned error: %v", err)
	}

	// Reload - should read the same values
	cfg2, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("second Load() returned error: %v", err)
	}

	// Assert values match
	if cfg1.Generation.DefaultLanguage != cfg2.Generation.DefaultLanguage {
		t.Errorf("round-trip failed: default_language changed from %q to %q", cfg1.Generation.DefaultLanguage, cfg2.Generation.DefaultLanguage)
	}
	if cfg1.Generation.MaxIterations != cfg2.Generation.MaxIterations {
		t.Errorf("round-trip failed: max_iterations changed from %d to %d", cfg1.Generation.MaxIterations, cfg2.Generation.MaxIterations)
	}
	if cfg1.Execution.Sandbox != cfg2.Execution.Sandbox {
		t.Errorf("round-trip failed: sandbox changed from %q to %q", cfg1.Execution.Sandbox, cfg2.Execution.Sandbox)
	}
	if cfg1.Execution.RequireReview != cfg2.Execution.RequireReview {
		t.Errorf("round-trip failed: require_review changed from %v to %v", cfg1.Execution.RequireReview, cfg2.Execution.RequireReview)
	}
	if cfg1.Cache.TagOverlapMinScore != cfg2.Cache.TagOverlapMinScore {
		t.Errorf("round-trip failed: tag_overlap_min_score changed from %f to %f", cfg1.Cache.TagOverlapMinScore, cfg2.Cache.TagOverlapMinScore)
	}
	if cfg1.Cache.SimilarityThresholdAccept != cfg2.Cache.SimilarityThresholdAccept {
		t.Errorf("round-trip failed: similarity_threshold_accept changed from %f to %f", cfg1.Cache.SimilarityThresholdAccept, cfg2.Cache.SimilarityThresholdAccept)
	}
	if cfg1.Cache.SimilarityThresholdPrompt != cfg2.Cache.SimilarityThresholdPrompt {
		t.Errorf("round-trip failed: similarity_threshold_prompt changed from %f to %f", cfg1.Cache.SimilarityThresholdPrompt, cfg2.Cache.SimilarityThresholdPrompt)
	}
	if cfg1.Cache.AskOnAmbiguousMatch != cfg2.Cache.AskOnAmbiguousMatch {
		t.Errorf("round-trip failed: ask_on_ambiguous_match changed from %v to %v", cfg1.Cache.AskOnAmbiguousMatch, cfg2.Cache.AskOnAmbiguousMatch)
	}
	if cfg1.Model.IntentParser != cfg2.Model.IntentParser {
		t.Errorf("round-trip failed: intent_parser changed from %q to %q", cfg1.Model.IntentParser, cfg2.Model.IntentParser)
	}
}
