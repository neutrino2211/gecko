package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Rule struct {
	Prefix   string   `json:"prefix"`
	Contains string   `json:"contains"`
	Specs    []string `json:"specs"`
}

type Config struct {
	Order           []string `json:"order"`
	ExcludePrefixes []string `json:"exclude_prefixes"`
	PrefixRules     []Rule   `json:"prefix_rules"`
	ContainsRules   []Rule   `json:"contains_rules"`
}

type Result struct {
	Path     string
	Expected string
	Current  string
	HasTag   bool
	Unmapped bool
	Changed  bool
	Err      error
}

const (
	tagPrefix  = "// spec:"
	maxScanTop = 40
	mapPath    = "spec/file-spec-map.json"
)

func main() {
	mode := "audit"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	if mode != "audit" && mode != "apply" && mode != "check" {
		fmt.Fprintf(os.Stderr, "usage: spec_tags [audit|apply|check]\n")
		os.Exit(2)
	}

	cfg, err := loadConfig(mapPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	files, err := trackedSourceFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to list tracked files: %v\n", err)
		os.Exit(1)
	}

	results := make([]Result, 0, len(files))
	for _, path := range files {
		res := processFile(path, cfg, mode == "apply")
		results = append(results, res)
	}

	var missing, mismatched, unmapped, changed, errs int
	for _, r := range results {
		if r.Err != nil {
			errs++
			continue
		}
		if r.Unmapped {
			unmapped++
			continue
		}
		if r.Changed {
			changed++
		}
		if !r.HasTag {
			missing++
			continue
		}
		if r.Current != r.Expected {
			mismatched++
		}
	}

	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(os.Stderr, "ERROR %s: %v\n", r.Path, r.Err)
			continue
		}
		if r.Unmapped {
			fmt.Printf("UNMAPPED %s\n", r.Path)
			continue
		}
		if !r.HasTag {
			fmt.Printf("MISSING %s\n", r.Path)
			continue
		}
		if r.Current != r.Expected {
			fmt.Printf("MISMATCH %s\n", r.Path)
		}
	}

	fmt.Printf(
		"summary: files=%d changed=%d missing=%d mismatched=%d unmapped=%d errors=%d\n",
		len(results), changed, missing, mismatched, unmapped, errs,
	)

	if mode == "check" {
		if missing > 0 || mismatched > 0 || unmapped > 0 || errs > 0 {
			os.Exit(1)
		}
	}
	if mode == "apply" {
		if unmapped > 0 || errs > 0 {
			os.Exit(1)
		}
	}
}

func loadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Order) == 0 {
		return nil, errors.New("config.order must not be empty")
	}
	return &cfg, nil
}

func trackedSourceFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files", "*.go", "*.gecko")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		normalized := filepath.ToSlash(line)
		if strings.HasSuffix(normalized, ".gecko.c") {
			continue
		}
		files = append(files, normalized)
	}
	sort.Strings(files)
	return files, nil
}

func processFile(path string, cfg *Config, apply bool) Result {
	res := Result{Path: path}
	if shouldExclude(path, cfg) {
		res.Unmapped = true
		return res
	}

	specs := specsForPath(path, cfg)
	if len(specs) == 0 {
		res.Unmapped = true
		return res
	}

	tag := tagPrefix + " " + strings.Join(specs, ", ")
	res.Expected = tag

	content, err := os.ReadFile(path)
	if err != nil {
		res.Err = err
		return res
	}

	currentTag, tagLine := findCurrentTag(content)
	res.Current = currentTag
	res.HasTag = tagLine >= 0

	if !apply {
		return res
	}

	if res.HasTag && res.Current == res.Expected {
		return res
	}

	updated, changed := applyTag(content, tag, tagLine)
	if !changed {
		return res
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		res.Err = err
		return res
	}
	res.Changed = true
	res.Current = tag
	res.HasTag = true
	return res
}

func shouldExclude(path string, cfg *Config) bool {
	for _, prefix := range cfg.ExcludePrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func specsForPath(path string, cfg *Config) []string {
	set := make(map[string]bool)

	for _, rule := range cfg.PrefixRules {
		if rule.Prefix != "" && strings.HasPrefix(path, rule.Prefix) {
			for _, spec := range rule.Specs {
				set[spec] = true
			}
		}
	}
	for _, rule := range cfg.ContainsRules {
		if rule.Contains != "" && hasBoundedToken(path, rule.Contains) {
			for _, spec := range rule.Specs {
				set[spec] = true
			}
		}
	}

	if len(set) == 0 {
		return nil
	}

	ordered := make([]string, 0, len(set))
	for _, spec := range cfg.Order {
		if set[spec] {
			ordered = append(ordered, spec)
		}
	}
	for spec := range set {
		found := false
		for _, orderedSpec := range cfg.Order {
			if orderedSpec == spec {
				found = true
				break
			}
		}
		if !found {
			ordered = append(ordered, spec)
		}
	}
	return ordered
}

func hasBoundedToken(path, token string) bool {
	quoted := regexp.QuoteMeta(token)
	re := regexp.MustCompile(`(^|[^A-Za-z0-9])` + quoted + `([^A-Za-z0-9]|$)`)
	return re.MatchString(path)
}

func findCurrentTag(content []byte) (string, int) {
	lines := strings.Split(string(content), "\n")
	max := maxScanTop
	if len(lines) < max {
		max = len(lines)
	}
	for i := 0; i < max; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, tagPrefix) {
			return trimmed, i
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			// Once real code starts, don't look further.
			break
		}
	}
	return "", -1
}

func applyTag(content []byte, tag string, tagLine int) ([]byte, bool) {
	original := string(content)
	lines := strings.Split(original, "\n")

	if tagLine >= 0 {
		if strings.TrimSpace(lines[tagLine]) == tag {
			return content, false
		}
		lines[tagLine] = tag
		return []byte(strings.Join(lines, "\n")), true
	}

	updated := make([]string, 0, len(lines)+2)
	updated = append(updated, tag, "")
	updated = append(updated, lines...)
	return []byte(strings.Join(updated, "\n")), true
}
