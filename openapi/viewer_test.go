package openapi

import (
	"strings"
	"testing"
)

func TestScalarViewerHTML(t *testing.T) {
	body := ScalarViewer().HTML(Config{Name: "api", Title: "A < B"}, "/docs/openapi.json?x=</script>")
	if !strings.Contains(body, "@scalar/api-reference") {
		t.Fatalf("missing scalar script: %s", body)
	}
	if !strings.Contains(body, "A &lt; B") {
		t.Fatalf("title not escaped: %s", body)
	}
	if strings.Contains(body, "?x=</script>") {
		t.Fatalf("spec url not escaped for script: %s", body)
	}
}

func TestViewerFuncNil(t *testing.T) {
	var viewer ViewerFunc
	if viewer.HTML(Config{}, "/openapi.json") != "" {
		t.Fatal("nil viewer func should render empty body")
	}
}
