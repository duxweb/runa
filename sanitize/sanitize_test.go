package sanitize

import (
	"strings"
	"testing"
)

func TestHTMLStrictRemovesUnsafeContent(t *testing.T) {
	input := `<p onclick="x">hi<script>alert(1)</script><img src="x" onerror="x"></p>`
	got := HTML(input)
	if strings.Contains(got, "script") || strings.Contains(got, "onclick") || strings.Contains(got, "onerror") || strings.Contains(got, "img") {
		t.Fatalf("unsafe output: %s", got)
	}
	if !strings.Contains(got, "hi") {
		t.Fatalf("missing text: %s", got)
	}
}

func TestRichTextAndMarkdownPolicies(t *testing.T) {
	rich := HTML(`<a href="javascript:alert(1)">bad</a><strong>ok</strong>`, RichText())
	if strings.Contains(rich, "javascript") || !strings.Contains(rich, "<strong>ok</strong>") {
		t.Fatalf("rich output: %s", rich)
	}
	markdown := HTML(`<pre><code class="language-go">fmt.Println()</code></pre>`, Markdown())
	if !strings.Contains(markdown, "language-go") {
		t.Fatalf("markdown output: %s", markdown)
	}
}

func TestTextURLAndCustomPolicy(t *testing.T) {
	if got := Text("<b>hi</b>\x00"); got != "hi" {
		t.Fatalf("text = %q", got)
	}
	if URL("javascript:alert(1)") != "" || URL("data:text/html,x") != "" {
		t.Fatal("unsafe url should be removed")
	}
	if URL("https://example.com/a") == "" || URL("/safe/path") == "" {
		t.Fatal("safe url should be kept")
	}
	Register("identity", policyFunc(func(input string) string { return input }))
	if HTML("<x>", Use("identity")) != "<x>" {
		t.Fatal("custom policy not used")
	}
}
