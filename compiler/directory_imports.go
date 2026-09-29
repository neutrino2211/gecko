package compiler

import (
	"github.com/neutrino2211/gecko/frontend"
	"github.com/neutrino2211/gecko/tokens"
)

func ResolveTypeFromDirectoryImports(file *tokens.File, name string) (*tokens.File, bool) {
	return frontend.ResolveTypeFromDirectoryImports(file, name)
}
func ResolveModuleTypeFromDirectoryImports(file *tokens.File, module, name string) (*tokens.File, bool) {
	return frontend.ResolveModuleTypeFromDirectoryImports(file, module, name)
}
func ResolveMethodFromDirectoryImports(file *tokens.File, name string) (*tokens.File, bool) {
	return frontend.ResolveMethodFromDirectoryImports(file, name)
}
