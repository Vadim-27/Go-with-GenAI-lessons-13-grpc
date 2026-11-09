package docs_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"libratrack/internal/docs"
)

func parseSpec(t *testing.T) map[string]any {
	t.Helper()

	var spec map[string]any
	if err := yaml.Unmarshal(docs.Spec, &spec); err != nil {
		t.Fatalf("openapi.yaml is not valid YAML: %v", err)
	}
	return spec
}

func TestSpec_DescribesEveryRoute(t *testing.T) {
	spec := parseSpec(t)

	if v, _ := spec["openapi"].(string); !strings.HasPrefix(v, "3.") {
		t.Errorf("openapi version = %q, want 3.x", v)
	}

	paths, _ := spec["paths"].(map[string]any)
	want := map[string][]string{
		"/health":                         {"get"},
		"/api/v1/categories":              {"get", "post"},
		"/api/v1/categories/{id}":         {"get"},
		"/api/v1/authors":                 {"get", "post"},
		"/api/v1/authors/{id}":            {"get"},
		"/api/v1/books":                   {"get", "post"},
		"/api/v1/books/{id}":              {"get", "put", "delete"},
		"/api/v1/books/{id}/copies":       {"get", "post"},
		"/api/v1/books/{id}/availability": {"get"},
		"/api/v1/auth/register":           {"post"},
		"/api/v1/auth/login":              {"post"},
		"/api/v1/me":                      {"get", "put"},
		"/api/v1/me/loans":                {"get"},
		"/api/v1/loans":                   {"post"},
		"/api/v1/loans/{id}/return":       {"post"},
		"/api/v1/users":                   {"get"},
		"/api/v1/users/{id}/role":         {"put"},
	}
	for path, methods := range want {
		item, ok := paths[path].(map[string]any)
		if !ok {
			t.Errorf("path %s is missing in the spec", path)
			continue
		}
		for _, method := range methods {
			if _, ok := item[method]; !ok {
				t.Errorf("%s %s is missing in the spec", strings.ToUpper(method), path)
			}
		}
	}
}

func TestSpec_AllRefsResolve(t *testing.T) {
	spec := parseSpec(t)

	var refs []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for key, val := range x {
				if s, ok := val.(string); ok && key == "$ref" {
					refs = append(refs, s)
				}
				walk(val)
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(spec)

	if len(refs) == 0 {
		t.Fatal("expected the spec to contain $ref links")
	}
	for _, ref := range refs {
		node := any(spec)
		for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			m, ok := node.(map[string]any)
			if !ok {
				t.Errorf("$ref %q does not resolve (stopped at %q)", ref, part)
				break
			}
			if node, ok = m[part]; !ok {
				t.Errorf("$ref %q does not resolve (no %q)", ref, part)
				break
			}
		}
	}
}
