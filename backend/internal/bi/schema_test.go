package bi

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestEmbeddedSchemasMatchOpenAPIVersion110(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/mobile-bi/openapi.yaml")
	require.NoError(t, err)
	var spec struct {
		Info struct {
			Version string `yaml:"version"`
		} `yaml:"info"`
		Components struct {
			Schemas map[string]any `yaml:"schemas"`
		} `yaml:"components"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &spec))
	require.Equal(t, "1.1.0", spec.Info.Version)
	components, err := json.Marshal(spec.Components.Schemas)
	require.NoError(t, err)
	var embedded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(schemasJSON, &embedded))
	var nested map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(embedded["components"], &nested))
	require.JSONEq(t, string(components), string(nested["schemas"]))
	_, err = compiledSchemas()
	require.NoError(t, err)
}

func TestImportSchemaPreservesNullableWatermarkAndTypedRecords(t *testing.T) {
	valid := `{"source_id":"source","schema_version":"1","expected_checkpoint":null,"checkpoint":"first","records":[],"complete_through":null,"history_start_date":"2026-09-01","initial_backfill_complete":false}`
	require.NoError(t, validateSchema("ImportBatch", []byte(valid)))
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(valid), &body))
	body["initial_backfill_complete"] = true
	raw, _ := json.Marshal(body)
	require.Error(t, validateSchema("ImportBatch", raw))
	body["initial_backfill_complete"] = false
	body["organization_id"] = "other-enterprise"
	raw, _ = json.Marshal(body)
	require.Error(t, validateSchema("ImportBatch", raw))
	delete(body, "organization_id")
	body["records"] = []any{map[string]any{"kind": "manager_grant", "revision": 1, "payload": map[string]any{}}}
	raw, _ = json.Marshal(body)
	require.Error(t, validateSchema("ImportBatch", raw))
}

func TestTokenNormalizationDoesNotDoubleCountCacheOrInventZero(t *testing.T) {
	ptr := func(s string) *string { return &s }
	for _, input := range []RawTokens{
		{Encoding: "exclusive_buckets", Input: ptr("100"), Output: ptr("20"), CacheRead: ptr("40"), CacheWrite: ptr("10")},
		{Encoding: "inclusive_input", Input: ptr("150"), Output: ptr("20"), CacheRead: ptr("40"), CacheWrite: ptr("10")},
	} {
		normalized, err := NormalizeTokens(input)
		require.NoError(t, err)
		require.Equal(t, "150", *normalized.Input)
		require.Equal(t, "20", *normalized.Output)
	}
	value, err := NormalizeTokens(RawTokens{Encoding: "unavailable"})
	require.NoError(t, err)
	require.Nil(t, value.Input)
	value, err = NormalizeTokens(RawTokens{Encoding: "exclusive_buckets", Input: ptr("9007199254740993"), Output: ptr("0"), CacheRead: ptr("1"), CacheWrite: ptr("0")})
	require.NoError(t, err)
	require.Equal(t, "9007199254740994", *value.Input)
	_, err = NormalizeTokens(RawTokens{Encoding: "inclusive_input", Input: ptr("10"), Output: ptr("0"), CacheRead: ptr("9"), CacheWrite: ptr("2")})
	require.Error(t, err)
	_, err = NormalizeTokens(RawTokens{Encoding: "unavailable", Input: ptr("0")})
	require.Error(t, err)
}
