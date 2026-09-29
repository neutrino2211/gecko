package docgen

import (
	"testing"

	"github.com/neutrino2211/gecko/tokens"
)

func TestExtractClassDocPreservesMarkdown(t *testing.T) {
	class := &tokens.Class{
		Name:       "Widget",
		DocComment: []string{"/// # Widget", "///", "/// **Bold** text", "/// ```gecko", "/// let x: Widget", "/// ```"},
	}
	item := ExtractClassDoc(class, "widget.gecko")
	want := "# Widget\n\n**Bold** text\n```gecko\nlet x: Widget\n```"
	if item.DocComment != want {
		t.Fatalf("documentation = %q, want %q", item.DocComment, want)
	}
}
