package backends

import (
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/llir/llvm/ir"
	"github.com/neutrino2211/gecko/ast"
	llvmbackend "github.com/neutrino2211/gecko/backends/llvm_backend"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

type LLVMBackend struct {
	impls    interfaces.BackendCodegenImplementations
	features *FeatureSet
}

func (b *LLVMBackend) Init() {
	b.impls = &llvmbackend.LLVMBackendImplementation{Backend: b}
	b.features = NewLLVMFeatureSet()
	llvmbackend.CurrentBackend = b
	llvmbackend.FuncCalls = make(map[string]*ir.InstCall)
	llvmbackend.Methods = make(map[string]*ast.Method)
}

func (b *LLVMBackend) ProcessEntries(entries []*tokens.Entry, scope *ast.Ast) {
	BackendProcessEntries(b, scope, entries)
}

func (b *LLVMBackend) GetImpls() interfaces.BackendCodegenImplementations {
	return b.impls
}

func (b *LLVMBackend) Features() interfaces.FeatureChecker {
	return b.features
}

func (b *LLVMBackend) Compile(c *interfaces.BackendConfig) *exec.Cmd {
	file := &ast.Ast{
		Scope: c.SourceFile.PackageName,
	}

	file.Init(errors.NewErrorScope(c.SourceFile.Name, c.SourceFile.Path, c.SourceFile.Content))
	file.Config = c.SourceFile.Config

	// Set up type suggestion provider for helpful error messages
	if c.SuggestionProvider != nil {
		file.SuggestionProvider = func(typeName string) string {
			return c.SuggestionProvider(typeName)
		}
	}

	// Track files already processed via lazy resolution to avoid duplication
	lazyResolvedFiles := make(map[string]*ast.Ast)

	// Helper to get or create a scope from a lazy-resolved file
	getOrCreateLazyScope := func(resolvedFile *tokens.File) *ast.Ast {
		if existingScope, ok := lazyResolvedFiles[resolvedFile.Path]; ok {
			return existingScope
		}

		// Create a scope using the file's package name
		resolvedScope := &ast.Ast{
			Scope:  resolvedFile.PackageName,
			Parent: nil,
		}
		resolvedScope.Init(errors.NewErrorScope(resolvedFile.Name, resolvedFile.Path, resolvedFile.Content))
		resolvedScope.Config = resolvedFile.Config

		// Process the resolved file's entries
		b.ProcessEntries(resolvedFile.Entries, resolvedScope)

		// Add as child of main file for symbol resolution
		file.Children[resolvedFile.PackageName] = resolvedScope

		// Cache for future lazy resolutions from the same file
		lazyResolvedFiles[resolvedFile.Path] = resolvedScope

		return resolvedScope
	}

	// Set up lazy type resolver for directory imports
	if c.LazyTypeResolver != nil {
		file.LazyResolver = func(typeName string) (*ast.Ast, bool) {
			resolvedFile, found := c.LazyTypeResolver(typeName)
			if !found {
				return nil, false
			}

			resolvedScope := getOrCreateLazyScope(resolvedFile)

			// Find and return the requested class
			if cls, ok := resolvedScope.Classes[typeName]; ok {
				return cls, true
			}

			return nil, false
		}
	}

	// Set up lazy method resolver for directory imports
	if c.LazyMethodResolver != nil {
		file.LazyMethodResolver = func(methodName string) (*ast.Method, bool) {
			resolvedFile, found := c.LazyMethodResolver(methodName)
			if !found {
				return nil, false
			}

			resolvedScope := getOrCreateLazyScope(resolvedFile)

			// Find and return the requested method
			if mth, ok := resolvedScope.Methods[methodName]; ok {
				return mth, true
			}

			return nil, false
		}
	}

	b.ProcessEntries(c.SourceFile.Entries, file)

	// Check for errors - bail early if any
	if file.ErrorScope.HasErrors() {
		return nil
	}

	info := llvmbackend.LLVMGetScopeInformation(file)

	llir := info.ProgramContext.Module.String()

	if c.Ctx.Bool("print-ir") {
		println(c.File + "\n" + strings.Repeat("_", len(c.File)) + "\n\n" + llir)
	}

	os.WriteFile(c.OutName, []byte(llir), 0o755)

	if c.Ctx.Bool("ir-only") {
		cmd := exec.Command("cp", c.OutName, ".")
		err := streamCommand(cmd)

		if err != nil {
			file.ErrorScope.NewCompileTimeError("LLIR copy", "Error copying LLVM IR "+err.Error(), lexer.Position{})
		}

		return nil
	}

	llcArgs := []string{"-filetype=obj"}

	if file.Config.Arch == "arm64" && file.Config.Platform == "darwin" {
		llcArgs = append(llcArgs, "--mtriple", "arm64-apple-darwin21.4.0")
	} else if file.Config.Vendor != "" {
		llcArgs = append(llcArgs, "--mtriple", file.Config.Arch+"-"+file.Config.Vendor+"-"+file.Config.Platform)
	}

	llcArgs = append(llcArgs, file.Config.Ctx.String("output")+"/"+c.OutName)

	cmd := exec.Command("llc", llcArgs...)

	return cmd
}
