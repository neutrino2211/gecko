package frontend

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/neutrino2211/gecko/config"
)

type sourceInput struct {
	content []byte
	err     error
}

func (r *Result) Matches(path, content string, cfg *config.CompileCfg) bool {
	if r == nil || normalizePath(path) != normalizePath(r.path) || content != r.content {
		return false
	}
	var project *config.ProjectConfig
	if cfg != nil {
		project = cfg.Project
	}
	if !reflect.DeepEqual(r.project, project) {
		return false
	}
	for path, listing := range r.directories {
		if directoryListing(path) != listing {
			return false
		}
	}
	for path, input := range r.inputs {
		if path == normalizePath(r.path) {
			continue
		}
		current, err := readSource(path, cfg)
		if (err == nil) != (input.err == nil) {
			return false
		}
		if err != nil && err.Error() != input.err.Error() {
			return false
		}
		if !bytes.Equal(current, input.content) {
			return false
		}
	}
	return true
}

func snapshotConfig(cfg *config.CompileCfg, inputs map[string]sourceInput) (*config.CompileCfg, func()) {
	sealed := false
	copy := config.CompileCfg{}
	if cfg != nil {
		copy = *cfg
	}
	copy.Project = cloneProject(copy.Project)
	copy.CFlags = append([]string(nil), copy.CFlags...)
	copy.CLFlags = append([]string(nil), copy.CLFlags...)
	copy.CObjects = append([]string(nil), copy.CObjects...)
	copy.ReadSource = func(path string) ([]byte, error) {
		key := normalizePath(path)
		if input, ok := inputs[key]; ok {
			return append([]byte(nil), input.content...), input.err
		}
		content, err := readSource(path, cfg)
		content = append([]byte(nil), content...)
		if !sealed {
			inputs[key] = sourceInput{content: content, err: err}
		}
		return append([]byte(nil), content...), err
	}
	return &copy, func() { sealed = true }
}

func directoryListing(path string) string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err.Error()
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) == ".gecko" {
			names = append(names, entry.Name())
		}
	}
	return strings.Join(names, "\x00")
}

func snapshotDirectories(inputs map[string]sourceInput) map[string]string {
	directories := make(map[string]string)
	for path := range inputs {
		if filepath.Base(path) != "mod.gecko" {
			continue
		}
		parent := filepath.Dir(path)
		if _, ok := directories[parent]; !ok {
			directories[parent] = directoryListing(parent)
		}
	}
	return directories
}

func cloneProject(project *config.ProjectConfig) *config.ProjectConfig {
	if project == nil {
		return nil
	}
	copy := cloneConfigValue(reflect.ValueOf(project))
	return copy.Interface().(*config.ProjectConfig)
}

func cloneConfigValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return value
		}
		copy := reflect.New(value.Type().Elem())
		copy.Elem().Set(cloneConfigValue(value.Elem()))
		return copy
	case reflect.Struct:
		copy := reflect.New(value.Type()).Elem()
		copy.Set(value)
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).IsExported() {
				copy.Field(i).Set(cloneConfigValue(value.Field(i)))
			}
		}
		return copy
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		copy := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			copy.SetMapIndex(iter.Key(), cloneConfigValue(iter.Value()))
		}
		return copy
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		copy := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			copy.Index(i).Set(cloneConfigValue(value.Index(i)))
		}
		return copy
	default:
		return value
	}
}
