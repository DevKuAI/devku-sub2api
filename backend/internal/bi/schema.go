package bi

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// schemasJSON is generated from docs/mobile-bi/openapi.yaml components.schemas.
//
//go:embed schemas.json
var schemasJSON []byte

var compiledSchemas = sync.OnceValues(func() (map[string]*jsonschema.Schema, error) {
	resource, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemasJSON))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	const url = "https://schemas.devku.invalid/bi-v1.json"
	if err := c.AddResource(url, resource); err != nil {
		return nil, err
	}
	result := map[string]*jsonschema.Schema{}
	var document struct {
		Components struct{ Schemas map[string]json.RawMessage }
	}
	if err := json.Unmarshal(schemasJSON, &document); err != nil {
		return nil, err
	}
	if len(document.Components.Schemas) == 0 {
		return nil, fmt.Errorf("BI schema bundle is empty")
	}
	for name := range document.Components.Schemas {
		schema, err := c.Compile(url + "#/components/schemas/" + name)
		if err != nil {
			return nil, err
		}
		result[name] = schema
	}
	return result, nil
})

func validateSchema(name string, raw []byte) error {
	if !utf8.Valid(raw) {
		return invalid("body", "JSON must be valid UTF-8")
	}
	schemas, err := compiledSchemas()
	if err != nil {
		return err
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return invalid("body", "Invalid JSON")
	}
	if !boundedJSONNumbers(value) {
		return invalid("body", "Numeric value exceeds supported precision")
	}
	schema, exists := schemas[name]
	if !exists {
		return fmt.Errorf("unknown internal BI schema %q", name)
	}
	if err := schema.Validate(value); err != nil {
		// Schema errors can include the rejected value, so never return raw diagnostics.
		return invalid("body", "Request does not match the API v1.1.0 schema")
	}
	return nil
}

func boundedJSONNumbers(value any) bool {
	switch typed := value.(type) {
	case json.Number:
		text := typed.String()
		if len(text) > 4096 {
			return false
		}
		if index := strings.IndexAny(text, "eE"); index >= 0 {
			exponent, err := strconv.ParseInt(text[index+1:], 10, 32)
			if err != nil || exponent < -4096 || exponent > 4096 {
				return false
			}
		}
	case map[string]any:
		for _, item := range typed {
			if !boundedJSONNumbers(item) {
				return false
			}
		}
	case []any:
		for _, item := range typed {
			if !boundedJSONNumbers(item) {
				return false
			}
		}
	}
	return true
}
