package intent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"sharp-tools/config"
)

// InputType represents the type of input
type InputType string

const (
	InputTypeFile InputType = "file"
	InputTypeText InputType = "text"
	InputTypeData InputType = "data"
	InputTypeNone InputType = "none"
)

// OutputType represents the type of output
type OutputType string

const (
	OutputTypeFile OutputType = "file"
	OutputTypeText OutputType = "text"
	OutputTypeData OutputType = "data"
	OutputTypeNone OutputType = "none"
)

// Parameter represents a parameter for the intent
type Parameter struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// StructuredIntent represents a parsed intent from the AI model
type StructuredIntent struct {
	Intent       string      `json:"intent"`
	InputType    InputType   `json:"input_type"`
	OutputType   OutputType  `json:"output_type"`
	Parameters   []Parameter `json:"parameters"`
	Tags         []string    `json:"tags"`
	CanonicalKey string      `json:"canonical_key"`
}

// ErrParseFailure is returned when the AI response cannot be parsed
var ErrParseFailure = errors.New("failed to parse AI response")

// Preferred tags for normalization (loaded from docs/tags.md hints)
var preferredTags = map[string]bool{
	// Image Operations
	"convert": true, "resize": true, "crop": true, "rotate": true, "flip": true,
	"mirror": true, "compress": true, "optimize": true, "strip-metadata": true,
	"watermark": true, "thumbnail": true, "upscale": true, "downscale": true,
	"grayscale": true, "blur": true, "sharpen": true, "denoise": true,
	// Video Operations
	"transcode": true, "trim-video": true, "split-video": true, "merge-video": true,
	"extract-frames": true, "extract-audio": true, "add-subtitles": true,
	// Audio Operations
	"transcode-audio": true, "trim-audio": true, "split-audio": true,
	"normalize": true, "amplify": true, "noise-reduce": true,
	// Text Operations
	"strip": true, "replace": true, "extract": true, "truncate": true,
	"lowercase": true, "uppercase": true, "title-case": true,
	// Data Operations
	"parse": true, "serialize": true, "validate": true, "transform": true,
	// Format Conversion
	"json-to-csv": true, "csv-to-json": true, "json-to-yaml": true,
	// Compression
	"decompress": true, "archive": true, "extract-archive": true,
	// Security
	"encrypt": true, "decrypt": true, "hash-file": true,
	// Image formats
	"jpg": true, "png": true, "gif": true, "webp": true, "svg": true, "pdf": true,
	// Data formats
	"json": true, "yaml": true, "csv": true, "xml": true,
}

// NormalizeTag normalizes a tag according to the rules
func NormalizeTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	// First replace underscores with hyphens, then collapse multiple spaces/underscores
	tag = strings.ReplaceAll(tag, "_", "-")
	tag = strings.ReplaceAll(tag, " ", "-")
	// Collapse multiple hyphens into one
	for strings.Contains(tag, "--") {
		tag = strings.ReplaceAll(tag, "--", "-")
	}
	return strings.Trim(tag, "-")
}

// NormalizeTags normalizes a list of tags
func NormalizeTags(tags []string) []string {
	seen := make(map[string]bool)
	var normalized []string

	for _, tag := range tags {
		norm := NormalizeTag(tag)
		if norm != "" && !seen[norm] {
			seen[norm] = true
			normalized = append(normalized, norm)
		}
	}
	return normalized
}

// DeriveCanonicalKey creates a canonical key from structured intent fields
func DeriveCanonicalKey(intent *StructuredIntent) string {
	var primaryOp string
	for _, tag := range intent.Tags {
		if preferredTags[tag] {
			primaryOp = tag
			break
		}
	}


	if primaryOp == "" && len(intent.Tags) > 0 {
		primaryOp = intent.Tags[0]
	}

	inputStr := string(intent.InputType)
	outputStr := string(intent.OutputType)
	if intent.InputType == InputTypeNone {
		inputStr = ""
	}
	if intent.OutputType == OutputTypeNone {
		outputStr = ""
	}

	var parts []string
	if primaryOp != "" {
		parts = append(parts, "operation:"+primaryOp)
	}
	if inputStr != "" {
		parts = append(parts, "input:"+inputStr)
	}
	if outputStr != "" {
		parts = append(parts, "output:"+outputStr)
	}

	return strings.Join(parts, " | ")
}

// getSystemPrompt returns the system prompt with tag normalization hints
func getSystemPrompt() string {
	return `You are an intent parser. Given a raw user input, parse it into a structured JSON response.

Tag Normalization Hints:
- All tags should be lowercase
- Use hyphens (-) instead of underscores or spaces
- Prefer tags from this list when they fit: convert, resize, crop, rotate, flip, mirror, compress, optimize, thumbnail, blur, sharpen, denoise, transcode, trim-video, split-video, merge-video, extract-frames, transcode-audio, normalize, strip, replace, extract, lowercase, uppercase, parse, serialize, validate, encrypt, decrypt, hash-file, json-to-csv, csv-to-json
- Image formats: jpg, png, gif, webp, svg, pdf, tiff, heic
- Data formats: json, yaml, csv, xml, toml

Respond with ONLY valid JSON in this exact format:
{"intent": "<action>", "input_type": "file|text|data|none", "output_type": "file|text|data|none", "parameters": [{"name": "<param_name>", "type": "<param_type>", "required": true|false}], "tags": ["<tag1>", "<tag2>"]}

Do not include any explanation or additional text.`
}

// Parse parses a raw input string into a StructuredIntent using Claude Haiku
func Parse(ctx context.Context, client *anthropic.Client, cfg config.ModelConfig, rawInput string) (*StructuredIntent, error) {
	msg, err := client.Messages.New(
		ctx,
		anthropic.MessageNewParams{
			Model:     anthropic.Model(cfg.IntentParser),
			MaxTokens: int64(cfg.MaxTokens),
			Temperature: param.NewOpt(float64(cfg.Temperature)),
			System: []anthropic.TextBlockParam{
				{Text: getSystemPrompt()},
			},
			Messages: []anthropic.MessageParam{
				{
					Role: anthropic.MessageParamRoleUser,
					Content: []anthropic.ContentBlockParamUnion{
						anthropic.NewTextBlock("Parse this: " + rawInput),
					},
				},
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	var responseText string
	if len(msg.Content) > 0 {
		responseText = msg.Content[0].Text
	}

	if responseText == "" {
		return nil, ErrParseFailure
	}

	return ParseResponse(responseText)
}

// ParseResponse parses a JSON string into a StructuredIntent
func ParseResponse(responseText string) (*StructuredIntent, error) {
	jsonStr := extractJSON(responseText)
	if jsonStr == "" {
		return nil, ErrParseFailure
	}

	var intent StructuredIntent
	if err := json.Unmarshal([]byte(jsonStr), &intent); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParseFailure, err)
	}

	intent.Tags = NormalizeTags(intent.Tags)
	intent.CanonicalKey = DeriveCanonicalKey(&intent)

	return &intent, nil
}

// extractJSON extracts JSON from a string that may contain surrounding text
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return s[start : end+1]
}
