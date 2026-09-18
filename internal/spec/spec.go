package spec

import _ "embed"

// YAML is the OpenAPI 3.1 document served at /openapi.yaml.
//
//go:embed openapi.yaml
var YAML []byte

const SwaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>hello API docs</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
<script>
window.onload = function () {
  SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#swagger-ui" });
};
</script>
</body>
</html>`
