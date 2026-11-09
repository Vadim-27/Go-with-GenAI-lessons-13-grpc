package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"libratrack/internal/docs"
)

// swaggerPage is the Swagger UI page. The browser loads the library itself
// from a CDN and takes the specification from our /openapi.yaml.
const swaggerPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>LibraTrack API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#swagger-ui" });
  </script>
</body>
</html>
`

// OpenAPI answers GET /openapi.yaml with the API specification.
func OpenAPI(c *gin.Context) {
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", docs.Spec)
}

// SwaggerUI answers GET /swagger and /swagger/index.html: interactive docs in the browser.
func SwaggerUI(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerPage))
}
