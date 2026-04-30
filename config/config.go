package config

// ModelConfig holds configuration for AI model interactions
type ModelConfig struct {
	// IntentParser is the model ID to use for intent parsing
	IntentParser string
	// MaxTokens is the maximum number of tokens to generate
	MaxTokens int
	// Temperature controls randomness (0-1)
	Temperature float64
}

// DefaultModelConfig returns the default model configuration
func DefaultModelConfig() ModelConfig {
	return ModelConfig{
		IntentParser: "claude-3-haiku-20240307",
		MaxTokens:    512,
		Temperature:  0,
	}
}
