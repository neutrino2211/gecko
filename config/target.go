package config

import (
	"fmt"
	"strings"
)

type CompileTarget struct {
	Key      string
	Arch     string
	Vendor   string
	Platform string
}

func ResolveCompileTarget(project *ProjectConfig, override, hostArch, hostPlatform string) (CompileTarget, error) {
	if project != nil && project.Build.DefaultTarget != "" {
		if _, err := parseCompileTarget(project.Build.DefaultTarget); err != nil {
			return CompileTarget{}, err
		}
	}
	key := override
	if key == "" && project != nil {
		key = project.Build.DefaultTarget
	}
	if key == "" {
		return CompileTarget{Arch: hostArch, Platform: hostPlatform}, nil
	}
	return parseCompileTarget(key)
}

func parseCompileTarget(key string) (CompileTarget, error) {
	parts := strings.Split(key, "-")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return CompileTarget{}, fmt.Errorf("invalid target %q: expected architecture-platform or architecture-vendor-platform", key)
	}
	target := CompileTarget{Key: key, Arch: parts[0]}
	if len(parts) == 2 {
		target.Platform = parts[1]
	} else {
		if parts[2] == "" {
			return CompileTarget{}, fmt.Errorf("invalid target %q: missing platform", key)
		}
		target.Vendor = parts[1]
		target.Platform = parts[2]
	}
	switch target.Arch {
	case "x86_64":
		target.Arch = "amd64"
	case "aarch64":
		target.Arch = "arm64"
	}
	return target, nil
}
