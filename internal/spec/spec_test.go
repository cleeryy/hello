package spec

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// Given: le document embarqué
// When: on le parse
// Then: c'est un OpenAPI 3.1 qui couvre toutes les routes montées.
func TestEmbeddedSpec(t *testing.T) {
	var doc struct {
		OpenAPI string `yaml:"openapi"`
		Info    struct {
			Title   string `yaml:"title"`
			Version string `yaml:"version"`
		} `yaml:"info"`
		Paths      map[string]map[string]any `yaml:"paths"`
		Components struct {
			Schemas map[string]any `yaml:"schemas"`
		} `yaml:"components"`
	}
	require.NoError(t, yaml.Unmarshal(YAML, &doc))
	require.Equal(t, "3.1.0", doc.OpenAPI)
	require.Equal(t, "hello", doc.Info.Title)
	require.NotEmpty(t, doc.Info.Version)
	for _, p := range []string{"/", "/health", "/ws", "/wake", "/wake/{macAddress}",
		"/devices", "/devices/{id}", "/devices/{id}/wake", "/openapi.yaml", "/docs"} {
		ops, ok := doc.Paths[p]
		require.True(t, ok, "route %s documentée", p)
		require.NotEmpty(t, ops, "route %s a au moins une opération", p)
		for method, raw := range ops {
			if method == "parameters" || method == "servers" || method == "$ref" {
				continue
			}
			op, ok := raw.(map[string]any)
			require.True(t, ok, "%s %s est une opération", method, p)
			resp, ok := op["responses"]
			require.True(t, ok, "%s %s a des responses", method, p)
			require.NotEmpty(t, resp, "%s %s a des responses non vides", method, p)
		}
	}
	for _, s := range []string{"Device", "Problem", "ValidationProblem", "StatusChange"} {
		require.Contains(t, doc.Components.Schemas, s, "schema %s défini", s)
	}
	require.Contains(t, string(YAML), "application/problem+json")
}
