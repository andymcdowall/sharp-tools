package intent

import (
	"errors"
	"strings"
	"testing"
)

// These tests verify the core parsing and normalization logic
// without requiring the full Anthropic SDK to be available

func TestCanonicalKeyDerivation(t *testing.T) {
	tests := []struct {
		name     string
		intent   *StructuredIntent
		wantKey  string
	}{
		{
			name: "basic image conversion",
			intent: &StructuredIntent{
				Intent:     "convert image",
				InputType:  InputTypeFile,
				OutputType: OutputTypeFile,
				Tags:       []string{"convert", "jpg", "png"},
			},
			wantKey: "operation:convert | input:file | output:file",
		},
		{
			name: "resize operation",
			intent: &StructuredIntent{
				Intent:     "resize image",
				InputType:  InputTypeFile,
				OutputType: OutputTypeFile,
				Tags:       []string{"resize", "image"},
			},
			wantKey: "operation:resize | input:file | output:file",
		},
		{
			name: "json parse operation",
			intent: &StructuredIntent{
				Intent:     "parse json",
				InputType:  InputTypeText,
				OutputType: OutputTypeData,
				Tags:       []string{"parse", "json"},
			},
			wantKey: "operation:parse | input:text | output:data",
		},
		{
			name: "none input type",
			intent: &StructuredIntent{
				Intent:     "generate uuid",
				InputType:  InputTypeNone,
				OutputType: OutputTypeText,
				Tags:       []string{"generate", "uuid"},
			},
			wantKey: "operation:generate | output:text",
		},
		{
			name: "no tags fallback",
			intent: &StructuredIntent{
				Intent:     "do something",
				InputType:  InputTypeText,
				OutputType: OutputTypeText,
				Tags:       []string{"custom-action"},
			},
			wantKey: "operation:custom-action | input:text | output:text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey := DeriveCanonicalKey(tt.intent)
			if gotKey != tt.wantKey {
				t.Errorf("DeriveCanonicalKey() = %q, want %q", gotKey, tt.wantKey)
			}
		})
	}
}

func TestTagNormalisation(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		wantTags []string
	}{
		{
			name:     "lowercase conversion",
			tags:     []string{"CONVERT", "RESIZE"},
			wantTags: []string{"convert", "resize"},
		},
		{
			name:     "space to hyphen",
			tags:     []string{"image convert", "video transcode"},
			wantTags: []string{"image-convert", "video-transcode"},
		},
		{
			name:     "underscore to hyphen",
			tags:     []string{"json_to_csv", "strip_metadata"},
			wantTags: []string{"json-to-csv", "strip-metadata"},
		},
		{
			name:     "duplicate removal",
			tags:     []string{"convert", "convert", "resize"},
			wantTags: []string{"convert", "resize"},
		},
		{
			name:     "mixed case and spaces",
			tags:     []string{"  JSON  to   YAML  ", "UPPER"},
			wantTags: []string{"json-to-yaml", "upper"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTags := NormalizeTags(tt.tags)
			if len(gotTags) != len(tt.wantTags) {
				t.Errorf("NormalizeTags() returned %d tags, want %d", len(gotTags), len(tt.wantTags))
				return
			}
			for i, want := range tt.wantTags {
				if gotTags[i] != want {
					t.Errorf("NormalizeTags()[%d] = %q, want %q", i, gotTags[i], want)
				}
			}
		})
	}
}

func TestEquivalentInputsProduceSameKey(t *testing.T) {
	// Two different phrasings that should produce the same canonical key
	jsonResp1 := `{"intent": "convert image", "input_type": "file", "output_type": "file", "parameters": [], "tags": ["convert", "png", "jpg"]}`
	jsonResp2 := `{"intent": "convert image", "input_type": "file", "output_type": "file", "parameters": [], "tags": ["CONVERT", "PNG", "JPG"]}`

	intent1, err := ParseResponse(jsonResp1)
	if err != nil {
		t.Fatalf("ParseResponse(1) failed: %v", err)
	}

	intent2, err := ParseResponse(jsonResp2)
	if err != nil {
		t.Fatalf("ParseResponse(2) failed: %v", err)
	}

	if intent1.CanonicalKey != intent2.CanonicalKey {
		t.Errorf("Equivalent inputs produced different keys: %q vs %q", intent1.CanonicalKey, intent2.CanonicalKey)
	}
}

func TestMalformedResponseError(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		wantErr   error
	}{
		{
			name:     "invalid JSON",
			response: "not valid json at all",
			wantErr:  ErrParseFailure,
		},
		{
			name:     "empty response",
			response: "",
			wantErr:  ErrParseFailure,
		},
		{
			name:     "partial JSON",
			response: "{\"intent\": \"test\",",
			wantErr:  ErrParseFailure,
		},
		{
			name:     "text before JSON",
			response: "Here is the result: {\"intent\": \"test\"}",
			wantErr:  nil, // Should extract JSON successfully
		},
		{
			name:     "text after JSON",
			response: "{\"intent\": \"test\"}",
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseResponse(tt.response)
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("ParseResponse() error = %v, want %v", err, tt.wantErr)
			}
			if tt.response == "" && err != ErrParseFailure {
				t.Errorf("empty response should return ErrParseFailure")
			}
		})
	}
}

func TestParameterExtraction(t *testing.T) {
	jsonResp := `{
		"intent": "resize image",
		"input_type": "file",
		"output_type": "file",
		"parameters": [
			{"name": "width", "type": "number", "required": true},
			{"name": "height", "type": "number", "required": true},
			{"name": "maintain_aspect", "type": "boolean", "required": false}
		],
		"tags": ["resize", "image"]
	}`

	intent, err := ParseResponse(jsonResp)
	if err != nil {
		t.Fatalf("ParseResponse() failed: %v", err)
	}

	if len(intent.Parameters) != 3 {
		t.Errorf("Expected 3 parameters, got %d", len(intent.Parameters))
	}

	// Check first parameter
	if intent.Parameters[0].Name != "width" {
		t.Errorf("Expected first parameter name 'width', got %q", intent.Parameters[0].Name)
	}
	if intent.Parameters[0].Type != "number" {
		t.Errorf("Expected first parameter type 'number', got %q", intent.Parameters[0].Type)
	}
	if !intent.Parameters[0].Required {
		t.Error("Expected first parameter to be required")
	}

	// Check third parameter is optional
	if intent.Parameters[2].Required {
		t.Error("Expected third parameter to be optional")
	}

	// Verify canonical key is derived
	if !strings.Contains(intent.CanonicalKey, "operation:resize") {
		t.Errorf("CanonicalKey should contain 'operation:resize', got %q", intent.CanonicalKey)
	}
}

// Test that the system prompt contains tag normalization hints
func TestSystemPromptContainsHints(t *testing.T) {
	prompt := getSystemPrompt()
	if !strings.Contains(prompt, "lowercase") {
		t.Error("System prompt should mention lowercase normalization")
	}
	if !strings.Contains(prompt, "hyphens") {
		t.Error("System prompt should mention hyphens")
	}
}

// Test empty tags list
func TestEmptyTags(t *testing.T) {
	intent := &StructuredIntent{
		Intent:     "do something",
		InputType: InputTypeText,
		OutputType: OutputTypeText,
		Tags:      []string{},
	}

	key := DeriveCanonicalKey(intent)
	// Should still have input/output but no operation
	if !strings.Contains(key, "input:text") {
		t.Errorf("Expected key to contain 'input:text', got %q", key)
	}
}

// Test with "none" types that should be omitted
func TestNoneTypesOmitted(t *testing.T) {
	intent := &StructuredIntent{
		Intent:     "generate password",
		InputType:  InputTypeNone,
		OutputType: OutputTypeNone,
		Tags:       []string{"generate", "password"},
	}

	key := DeriveCanonicalKey(intent)
	// Should only have operation
	if key != "operation:generate" {
		t.Errorf("Expected key 'operation:generate', got %q", key)
	}
}
