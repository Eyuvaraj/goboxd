package docs

import (
	_ "embed"
	"net/http"
)

//go:embed swagger.json
var swaggerJSON []byte

// UIHandler serves the Swagger UI page. Mount at GET /docs/.
func UIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerUIPage))
}

// JSONHandler serves the embedded OpenAPI spec. Mount at GET /docs/swagger.json.
func JSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(swaggerJSON)
}

const swaggerUIPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>goboxd API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
  SwaggerUIBundle({
    url: '/docs/swagger.json',
    dom_id: '#swagger-ui',
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIBundle.SwaggerUIStandalonePreset,
    ],
    layout: 'BaseLayout',
    deepLinking: true,
  });
</script>
</body>
</html>`
