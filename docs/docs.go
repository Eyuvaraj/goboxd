// Package docs serves the OpenAPI/Swagger spec for the goboxd API.
//
// The single source of truth is swagger.json (also served verbatim at
// GET /docs/swagger.json by handler.go). This file embeds it so the swaggo/swag
// registry stays in sync automatically — there is no hand-maintained copy of the
// spec in Go anymore. swagger.yaml is a human-readable mirror of the same file.
package docs

import (
	_ "embed"

	"github.com/swaggo/swag"
)

//go:embed swagger.json
var docTemplate string

// SwaggerInfo registers the embedded spec with swaggo/swag.
var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "",
	BasePath:         "",
	Schemes:          []string{},
	Title:            "goboxd API",
	Description:      "Sandboxed code execution service. See swagger.json for the full contract.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
