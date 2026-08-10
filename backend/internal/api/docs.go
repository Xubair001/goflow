package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPISpec []byte

func (s *Server) handleOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	if _, err := w.Write(openAPISpec); err != nil {
		s.logger.Error("write openapi spec", "error", err)
	}
}

// swaggerUIPage loads Swagger UI from a CDN rather than vendoring it: this
// project's principle is one Go binary for the API + embedded SPA (see
// cmd/apiserver), and pulling in swagger-ui-dist's npm package just to
// render a docs page would be a disproportionate amount of bundled JS for
// what's a debugging aid, not a runtime dependency. It just needs the
// browser to have internet access.
const swaggerUIPage = `<!DOCTYPE html>
<html>
<head>
  <title>GoFlow API Docs</title>
  <meta charset="utf-8" />
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui",
      });
    };
  </script>
</body>
</html>
`

func (s *Server) handleSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(swaggerUIPage)); err != nil {
		s.logger.Error("write swagger ui page", "error", err)
	}
}
