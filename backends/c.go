package backends

import (
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/participle/lexer"
	"github.com/neutrino2211/gecko/ast"
	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

type CBackend struct {
	impls interfaces.BackendCodegenImplementations
}

func (b *CBackend) Init() {
	b.impls = &cbackend.CBackendImplementation{Backend: b}
	cbackend.CurrentBackend = b
	cbackend.Methods = make(map[string]*ast.Method)
}

func (b *CBackend) ProcessEntries(entries []*tokens.Entry, scope *ast.Ast) {
	BackendProcessEntries(b, scope, entries)
}

// generateGenericInstantiations generates C code for all pending generic instantiations
func (b *CBackend) generateGenericInstantiations(scope *ast.Ast) {
	impl := b.impls.(*cbackend.CBackendImplementation)

	// Generate class instantiations (struct + methods)
	for _, inst := range cbackend.Generics.ClassInstantiations {
		classToken, ok := cbackend.Generics.GenericClasses[inst.Name]
		if !ok {
			continue
		}
		impl.GenerateClassDef(scope, classToken, inst.FullName, inst.TypeArgs)

		// Also generate methods for this class instantiation
		for _, f := range classToken.Fields {
			if f.Method != nil {
				methodName := inst.FullName + "__" + f.Method.Name
				impl.GenerateClassMethodDef(scope, classToken, f.Method, methodName, inst.FullName, inst.TypeArgs)
			}
		}
	}

	// Generate method instantiations
	for _, inst := range cbackend.Generics.MethodInstantiations {
		methodToken, ok := cbackend.Generics.GenericMethods[inst.Name]
		if !ok {
			continue
		}

		// Validate trait constraints
		for i, param := range methodToken.TypeParams {
			if param.Trait != "" && i < len(inst.TypeArgs) {
				concreteType := inst.TypeArgs[i]
				// Remove pointer suffix for class lookup
				baseType := concreteType
				if len(baseType) > 0 && baseType[len(baseType)-1] == '*' {
					baseType = baseType[:len(baseType)-1]
				}
				// Also remove "struct " prefix if present
				if strings.HasPrefix(baseType, "struct ") {
					baseType = baseType[7:]
				}

				// Look up the class and check if it implements the trait
				classOpt := scope.ResolveClass(baseType)
				if !classOpt.IsNil() {
					class := classOpt.Unwrap()
					_, hasTrait := class.Traits[param.Trait]
					if !hasTrait {
						scope.ErrorScope.NewCompileTimeError(
							"Trait Constraint Error",
							"Type '"+baseType+"' does not implement trait '"+param.Trait+"' required by type parameter '"+param.Name+"'",
							lexer.Position{},
						)
					}
				}
			}
		}

		impl.GenerateMethodDef(scope, methodToken, inst.FullName, inst.TypeArgs)
	}

	// Clear for next compilation
	cbackend.ResetGenerics()
}

func (b *CBackend) GetImpls() interfaces.BackendCodegenImplementations {
	return b.impls
}

func (b *CBackend) Compile(c *interfaces.BackendConfig) *exec.Cmd {
	file := &ast.Ast{
		Scope: c.SourceFile.PackageName,
	}

	file.Init(errors.NewErrorScope(c.SourceFile.Name, c.SourceFile.Path, c.SourceFile.Content))
	file.Config = c.SourceFile.Config

	// Track all import scopes for merging
	importScopes := make([]*ast.Ast, 0)

	// Process imported modules first (as root-level scopes for proper naming)
	for _, importedFile := range c.SourceFile.Imports {
		importScope := &ast.Ast{
			Scope:  importedFile.PackageName,
			Parent: nil, // No parent so names are module__symbol, not main__module__symbol
		}
		importScope.Init(errors.NewErrorScope(importedFile.Name, importedFile.Path, importedFile.Content))
		importScope.Config = importedFile.Config

		// Add import scope as child of main file for symbol resolution
		file.Children[importedFile.PackageName] = importScope

		b.ProcessEntries(importedFile.Entries, importScope)
		importScopes = append(importScopes, importScope)
	}

	b.ProcessEntries(c.SourceFile.Entries, file)

	// Generate all pending generic instantiations
	b.generateGenericInstantiations(file)

	// Print any errors from the backend error scope
	if file.ErrorScope.HasErrors() {
		for _, err := range file.ErrorScope.CompileTimeErrors {
			println(err.GetError())
		}
	}

	info := cbackend.CGetScopeInformation(file)

	// Merge imported module info before main info (imports come first)
	for _, importScope := range importScopes {
		importInfo := cbackend.CGetScopeInformation(importScope)
		// Prepend import info so definitions come before usage
		info.TypeDefs = append(importInfo.TypeDefs, info.TypeDefs...)
		info.Types = append(importInfo.Types, info.Types...)
		info.Declarations = append(importInfo.Declarations, info.Declarations...)
		info.Globals = append(importInfo.Globals, info.Globals...)
		info.Functions = append(importInfo.Functions, info.Functions...)
	}

	// Generate C code
	cCode := generateCCode(info)

	if c.Ctx.Bool("print-ir") {
		println(c.File + "\n" + strings.Repeat("_", len(c.File)) + "\n\n" + cCode)
	}

	// Write output file with .c extension
	// c.OutName already contains the build directory path (e.g., /tmp/gecko/build/xxx/file.ll)
	// Replace .ll extension with .c
	outFile := c.OutName
	if strings.HasSuffix(outFile, ".ll") {
		outFile = outFile[:len(outFile)-3] + ".c"
	} else if idx := strings.LastIndex(outFile, "."); idx != -1 {
		outFile = outFile[:idx] + ".c"
	} else {
		outFile = outFile + ".c"
	}

	// Ensure the parent directory exists
	if idx := strings.LastIndex(outFile, "/"); idx != -1 {
		parentDir := outFile[:idx]
		os.MkdirAll(parentDir, 0o755)
	}

	err := os.WriteFile(outFile, []byte(cCode), 0o755)
	if err != nil {
		file.ErrorScope.NewCompileTimeError("File write error", "Error writing C file: "+err.Error(), lexer.Position{})
		return nil
	}

	if c.Ctx.Bool("ir-only") {
		// Copy to same directory as source file (not current dir, to avoid Go finding .c files)
		sourceDir := "."
		if idx := strings.LastIndex(c.File, "/"); idx != -1 {
			sourceDir = c.File[:idx]
		}
		destFile := sourceDir + "/" + c.File[strings.LastIndex(c.File, "/")+1:] + ".c"

		cmd := exec.Command("cp", outFile, destFile)
		streamErr := streamCommand(cmd)

		if streamErr != nil {
			file.ErrorScope.NewCompileTimeError("C copy", "Error copying C file "+streamErr.Error(), lexer.Position{})
		}

		return nil
	}

	// Compile with gcc
	gccArgs := []string{"-c", "-ffreestanding", "-nostdlib"}

	// Add architecture-specific flags
	if file.Config.Arch == "arm64" && file.Config.Platform == "darwin" {
		gccArgs = append(gccArgs, "-target", "arm64-apple-darwin")
	} else if file.Config.Vendor != "" {
		gccArgs = append(gccArgs, "-target", file.Config.Arch+"-"+file.Config.Vendor+"-"+file.Config.Platform)
	}

	// Output object file
	objFile := outFile[:len(outFile)-2] + ".o"
	gccArgs = append(gccArgs, "-o", objFile)
	gccArgs = append(gccArgs, outFile)

	cmd := exec.Command("gcc", gccArgs...)

	return cmd
}

// generateCCode creates the final C source code from scope information
func generateCCode(info *cbackend.CScopeInformation) string {
	var sb strings.Builder

	// Header comment
	sb.WriteString("/* Generated by Gecko C Backend */\n\n")

	// Include stdint.h for standard integer types
	sb.WriteString("#include <stdint.h>\n\n")

	// Write typedef declarations (external types)
	if len(info.TypeDefs) > 0 {
		sb.WriteString("/* External type declarations */\n")
		for _, typeDef := range info.TypeDefs {
			sb.WriteString(typeDef)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Write type definitions (structs)
	if len(info.Types) > 0 {
		sb.WriteString("/* Type definitions */\n")
		for _, typeDef := range info.Types {
			sb.WriteString(typeDef)
			sb.WriteString("\n")
		}
	}

	// Write extern declarations
	if len(info.Declarations) > 0 {
		sb.WriteString("/* External declarations */\n")
		for _, decl := range info.Declarations {
			sb.WriteString(decl)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Write global variables
	if len(info.Globals) > 0 {
		sb.WriteString("/* Global variables */\n")
		for _, global := range info.Globals {
			sb.WriteString(global)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Write function definitions
	if len(info.Functions) > 0 {
		sb.WriteString("/* Function definitions */\n")
		for _, fn := range info.Functions {
			sb.WriteString(fn)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
