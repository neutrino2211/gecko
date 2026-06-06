package commands

import (
	"flag"
	"reflect"
	"testing"

	"github.com/neutrino2211/gecko/config"
	"github.com/urfave/cli/v2"
)

func boolPtr(v bool) *bool { return &v }

func newTreeshakeTestContext(t *testing.T, args []string) *cli.Context {
	t.Helper()
	fs := flag.NewFlagSet("treeshake-test", flag.ContinueOnError)
	_ = fs.Bool("treeshake", false, "")
	_ = fs.Bool("no-treeshake", false, "")
	_ = fs.String("target-platform", "", "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("failed to parse args %v: %v", args, err)
	}
	return cli.NewContext(cli.NewApp(), fs, nil)
}

func TestResolveTreeshakeEnabledPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		config *config.ProjectConfig
		want   bool
	}{
		{
			name: "default_enabled_when_unset",
			want: true,
		},
		{
			name: "project_config_false",
			config: &config.ProjectConfig{
				Build: config.BuildConfig{Treeshake: boolPtr(false)},
			},
			want: false,
		},
		{
			name: "project_config_true",
			config: &config.ProjectConfig{
				Build: config.BuildConfig{Treeshake: boolPtr(true)},
			},
			want: true,
		},
		{
			name: "explicit_disable_overrides_project_true",
			args: []string{"--no-treeshake"},
			config: &config.ProjectConfig{
				Build: config.BuildConfig{Treeshake: boolPtr(true)},
			},
			want: false,
		},
		{
			name: "explicit_enable_overrides_project_false",
			args: []string{"--treeshake"},
			config: &config.ProjectConfig{
				Build: config.BuildConfig{Treeshake: boolPtr(false)},
			},
			want: true,
		},
		{
			name: "explicit_enable_wins_when_both_flags_set",
			args: []string{"--no-treeshake", "--treeshake"},
			config: &config.ProjectConfig{
				Build: config.BuildConfig{Treeshake: boolPtr(false)},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newTreeshakeTestContext(t, tc.args)
			got := resolveTreeshakeEnabled(ctx, tc.config)
			if got != tc.want {
				t.Fatalf("resolveTreeshakeEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTreeshakeLinkerFlagsForPlatform(t *testing.T) {
	if got := treeshakeLinkerFlagsForPlatform("darwin", true); !reflect.DeepEqual(got, []string{"-Wl,-dead_strip"}) {
		t.Fatalf("darwin flags = %v", got)
	}
	if got := treeshakeLinkerFlagsForPlatform("linux", true); !reflect.DeepEqual(got, []string{"-Wl,--gc-sections"}) {
		t.Fatalf("linux flags = %v", got)
	}
	if got := treeshakeLinkerFlagsForPlatform("windows", true); len(got) != 0 {
		t.Fatalf("windows flags should be empty, got %v", got)
	}
	if got := treeshakeLinkerFlagsForPlatform("linux", false); len(got) != 0 {
		t.Fatalf("disabled flags should be empty, got %v", got)
	}
}

func TestAddTreeshakeCompileFlags(t *testing.T) {
	base := []string{"-Wall"}
	enabled := addTreeshakeCompileFlags(append([]string{}, base...), true)
	if !reflect.DeepEqual(enabled, []string{"-Wall", "-ffunction-sections", "-fdata-sections"}) {
		t.Fatalf("enabled compile flags = %v", enabled)
	}

	disabled := addTreeshakeCompileFlags(append([]string{}, base...), false)
	if !reflect.DeepEqual(disabled, base) {
		t.Fatalf("disabled compile flags = %v", disabled)
	}
}
