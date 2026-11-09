// Package docs holds the OpenAPI specification embedded into the binary.
package docs

import _ "embed"

// Spec is the content of openapi.yaml. The go:embed directive reads the file
// at build time, so no separate file is needed next to the program.
//
//go:embed openapi.yaml
var Spec []byte
