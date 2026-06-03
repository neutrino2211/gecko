package tests

import (
	"reflect"
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/config"
)

func containsAll(t *testing.T, got []string, want []string) {
	t.Helper()
	joined := " " + strings.Join(got, " ") + " "
	for _, w := range want {
		if !strings.Contains(joined, " "+w+" ") {
			t.Fatalf("missing expected flag %q in %v", w, got)
		}
	}
}

func TestProjectConfigNativeFlagsAndPaths(t *testing.T) {
	cfg := &config.ProjectConfig{
		ProjectRoot: "/tmp/gecko-project",
		Build: config.BuildConfig{
			CFlags:  []string{"-Wall"},
			LdFlags: []string{"-lm"},
			Native: &config.NativeConfig{
				Headers:     []string{"<stdio.h>", "mylib.h"},
				IncludeDirs: []string{"include"},
				LibDirs:     []string{"lib"},
				Libs:        []string{"swiftlib"},
				Objects:     []string{"build/swiftlib.o"},
				CFlags:      []string{"-DAPP_NATIVE=1"},
				LdFlags:     []string{"-lpthread"},
			},
		},
		Targets: map[string]*config.TargetConfig{
			"x86_64-unknown-linux-gnu": {
				Native: &config.NativeConfig{
					Headers:     []string{"linux_only.h"},
					IncludeDirs: []string{"linux/include"},
					Objects:     []string{"linux/native.o"},
				},
			},
		},
	}

	cflags, err := cfg.GetCFlagsForTarget("x86_64-unknown-linux-gnu")
	if err != nil {
		t.Fatalf("GetCFlagsForTarget failed: %v", err)
	}
	containsAll(t, cflags, []string{
		"-Wall",
		"-DAPP_NATIVE=1",
		"-I/tmp/gecko-project/include",
		"-I/tmp/gecko-project/linux/include",
	})

	ldflags, err := cfg.GetLdFlagsForTarget("x86_64-unknown-linux-gnu", false)
	if err != nil {
		t.Fatalf("GetLdFlagsForTarget failed: %v", err)
	}
	containsAll(t, ldflags, []string{
		"-lm",
		"-lpthread",
		"-L/tmp/gecko-project/lib",
		"-lswiftlib",
	})

	headers := cfg.GetNativeHeadersForTarget("x86_64-unknown-linux-gnu")
	containsAll(t, headers, []string{
		"#include <stdio.h>",
		"#include \"mylib.h\"",
		"#include \"linux_only.h\"",
	})

	objects := cfg.GetNativeObjectsForTarget("x86_64-unknown-linux-gnu")
	containsAll(t, objects, []string{
		"/tmp/gecko-project/build/swiftlib.o",
		"/tmp/gecko-project/linux/native.o",
	})
}

func TestProjectConfigPreservesPairedLinkerFlags(t *testing.T) {
	cfg := &config.ProjectConfig{
		ProjectRoot: "/tmp/gecko-project",
		Build: config.BuildConfig{
			Native: &config.NativeConfig{
				LdFlags: []string{"-framework", "WebKit", "-framework", "Cocoa"},
			},
		},
	}

	ldflags, err := cfg.GetLdFlagsForTarget("", false)
	if err != nil {
		t.Fatalf("GetLdFlagsForTarget failed: %v", err)
	}

	want := []string{"-framework", "WebKit", "-framework", "Cocoa"}
	if !reflect.DeepEqual(ldflags, want) {
		t.Fatalf("expected linker flags %v, got %v", want, ldflags)
	}
}
