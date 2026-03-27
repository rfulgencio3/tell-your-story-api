package docs

import (
	"embed"
	"io/fs"
	"net/http"
)

// assets contains the embedded OpenAPI spec and Swagger UI files.
//
//go:embed openapi.yaml swagger/index.html
var assets embed.FS

// Handler serves the embedded Swagger UI and OpenAPI spec.
func Handler() (http.Handler, error) {
	subtree, err := fs.Sub(assets, ".")
	if err != nil {
		return nil, err
	}

	return http.FileServer(http.FS(subtree)), nil
}
