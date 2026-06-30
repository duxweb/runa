package embedgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.html"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}
	body, err := Generate(Config{Name: "ViewFS", Root: dir, Patterns: []string{"**/*.html"}, Package: "embed"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	output := string(body)
	if !strings.Contains(output, "package embed") || !strings.Contains(output, "var ViewFS embed.FS") || !strings.Contains(output, "a.html") {
		t.Fatalf("output = %s", output)
	}
	if strings.Contains(output, "b.txt") {
		t.Fatalf("unexpected txt = %s", output)
	}
}
