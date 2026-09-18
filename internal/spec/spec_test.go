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
		Security   []map[string]any          `yaml:"security"`
		Components struct {
			Schemas         map[string]any `yaml:"schemas"`
			SecuritySchemes map[string]struct {
				Type   string `yaml:"type"`
				Scheme string `yaml:"scheme"`
			} `yaml:"securitySchemes"`
			Responses map[string]any `yaml:"responses"`
		} `yaml:"components"`
	}
	require.NoError(t, yaml.Unmarshal(YAML, &doc))
	require.Equal(t, "3.1.0", doc.OpenAPI)
	require.Equal(t, "hello", doc.Info.Title)
	require.NotEmpty(t, doc.Info.Version)
	paths := []string{
		"/", "/health", "/ws", "/wake", "/wake/{macAddress}",
		"/devices", "/devices/{id}", "/devices/{id}/wake", "/openapi.yaml", "/docs",
	}
	for _, p := range paths {
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

// Given: le document embarqué
// When: on inspecte sa sécurité
// Then: bearerAuth global, 401 sur les opérations protégées, exemptions publiques.
func TestEmbeddedSpecSecurity(t *testing.T) {
	var doc struct {
		Security   []map[string]any          `yaml:"security"`
		Paths      map[string]map[string]any `yaml:"paths"`
		Components struct {
			SecuritySchemes map[string]struct {
				Type   string `yaml:"type"`
				Scheme string `yaml:"scheme"`
			} `yaml:"securitySchemes"`
			Responses map[string]any `yaml:"responses"`
		} `yaml:"components"`
	}
	require.NoError(t, yaml.Unmarshal(YAML, &doc))

	require.Len(t, doc.Security, 1, "sécurité globale définie")
	require.Contains(t, doc.Security[0], "bearerAuth")
	scheme, ok := doc.Components.SecuritySchemes["bearerAuth"]
	require.True(t, ok, "scheme bearerAuth défini")
	require.Equal(t, "http", scheme.Type)
	require.Equal(t, "bearer", scheme.Scheme)
	require.Contains(t, doc.Components.Responses, "Unauthorized")

	public := map[string]bool{"/": true, "/health": true, "/openapi.yaml": true, "/docs": true}
	for path, ops := range doc.Paths {
		for method, raw := range ops {
			op, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			responses, _ := op["responses"].(map[string]any)
			if public[path] {
				require.NotContains(t, responses, "401", "%s %s public sans 401", method, path)
				continue
			}
			if method == "parameters" {
				continue
			}
			require.Contains(t, responses, "401", "%s %s protégé documente 401", method, path)
		}
	}
}
