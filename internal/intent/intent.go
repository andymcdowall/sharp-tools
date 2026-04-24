package intent

// Parameter represents a tool parameter
type Parameter struct {
	Name     string
	Type     string // "filepath" | "string" | "integer" | "boolean"
	Required bool
}

// StructuredIntent represents a parsed user intent
type StructuredIntent struct {
	Intent       string
	InputType    string // "file" | "text" | "data" | "none"
	OutputType   string // "file" | "text" | "data" | "none"
	Parameters   []Parameter
	Tags         []string
	CanonicalKey string
}