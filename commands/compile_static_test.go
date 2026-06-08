package commands

import (
	"strings"
	"testing"
)

func TestValidateStaticLinkRequest(t *testing.T) {
	t.Run("allows non-static", func(t *testing.T) {
		if err := validateStaticLinkRequest("darwin", false); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("allows static linux", func(t *testing.T) {
		if err := validateStaticLinkRequest("linux", true); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("rejects static darwin", func(t *testing.T) {
		err := validateStaticLinkRequest("darwin", true)
		if err == nil {
			t.Fatalf("expected static darwin validation error")
		}
		msg := err.Error()
		if !strings.Contains(msg, "crt0.o") {
			t.Fatalf("expected error to mention crt0.o, got %q", msg)
		}
	})
}

