package frontend_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/frontend"
)

func TestCoherenceDiagnosticsWithoutBackend(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GECKO_HOME", root)
	cases := []struct {
		name    string
		message string
	}{
		{"inherent_foreign_type_error", "cannot add inherent impl for foreign type"},
		{"trait_impl_foreign_foreign_error", "orphan impl is not allowed"},
		{"generic_orphan_error", "orphan impl is not allowed"},
		{"generic_inherent_foreign_error", "cannot add inherent impl for foreign type"},
		{"trait_impl_local_trait_foreign_type_ok", ""},
		{"trait_impl_foreign_trait_local_type_ok", ""},
	}
	session := frontend.NewSession()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join("..", "test_sources", "compile_tests", "coherence", test.name+".gecko")
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			cached := session.Analyze(path, string(content), nil)
			if cached.ParseError() != nil {
				t.Fatal(cached.ParseError())
			}
			fresh := frontend.Analyze(path, string(content), nil)
			if !reflect.DeepEqual(cached.View().Program.Diagnostics(), fresh.View().Program.Diagnostics()) {
				t.Fatal("cached and fresh coherence diagnostics differ")
			}
			var found int
			for _, diagnostic := range cached.View().Program.Diagnostics() {
				if diagnostic.Code != errors.CodeCoherence {
					continue
				}
				found++
				if !strings.Contains(diagnostic.Message, test.message) || diagnostic.Pos.Line == 0 || diagnostic.EndOffset <= diagnostic.Pos.Offset || diagnostic.Help == "" {
					t.Fatalf("incomplete coherence diagnostic: %#v", diagnostic)
				}
			}
			if test.message == "" && found != 0 || test.message != "" && found != 1 {
				t.Fatalf("got %d coherence diagnostics, want one iff invalid; diagnostics=%#v", found, cached.View().Program.Diagnostics())
			}
		})
	}
}
