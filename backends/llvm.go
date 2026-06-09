// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
	"github.com/neutrino2211/gecko/ast"
	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
	llvmbackend "github.com/neutrino2211/gecko/backends/llvm_backend"
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
	llvmbackend.LLVMExecutionContext = nil
	llvmbackend.LLVMScopeDataMap = &llvmbackend.LLVMScopeData{}
	llvmbackend.LLVMProgramValues = &llvmbackend.LLVMValuesMap{}
	llvmbackend.LLVMStructMap = make(map[string]*llvmbackend.LLVMStructInfo)
	llvmbackend.LLVMOpaqueTypeMap = make(map[string]*types.StructType)
	llvmbackend.LLVMEnumMap = make(map[string]*llvmbackend.LLVMEnumInfo)
	llvmbackend.TraitDefinitionOrigins = make(map[string]string)
	llvmbackend.TraitParents = make(map[string]string)
	cbackend.InitTypeParameterChecker()
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
	sharedResult := PrepareSharedCompilePipeline(b, c, SharedCompilePipelineOptions{
		ProcessImports:            true,
		MarkImportedModules:       true,
		TrackLazyResolvedAsImport: false,
	})
	file := sharedResult.RootScope
	importScopes := sharedResult.ImportScopes

	for _, importScope := range importScopes {
		if importScope.ErrorScope.HasErrors() {
			return nil
		}
	}

	// Check for errors - bail early if any
	if file.ErrorScope.HasErrors() {
		return nil
	}

	info := llvmbackend.LLVMGetScopeInformation(file)
	info.ProgramContext.EmitExternalRootAnchors()

	// Set the target triple on the LLVM IR module
	if file.Config != nil {
		triple := buildTargetTriple(file.Config.Arch, file.Config.Vendor, file.Config.Platform)
		info.ProgramContext.Module.TargetTriple = triple
	}

	llir := info.ProgramContext.Module.String()

	if c.Ctx.Bool("print-ir") {
		println(c.File + "\n" + strings.Repeat("_", len(c.File)) + "\n\n" + llir)
	}

	if err := os.MkdirAll(filepath.Dir(c.OutName), 0o755); err != nil {
		file.ErrorScope.NewCompileTimeError("LLVM output", "Error creating LLVM output directory "+err.Error(), lexer.Position{})
		return nil
	}
	if err := os.WriteFile(c.OutName, []byte(llir), 0o755); err != nil {
		file.ErrorScope.NewCompileTimeError("LLVM output", "Error writing LLVM IR "+err.Error(), lexer.Position{})
		return nil
	}

	if c.Ctx.Bool("ir-only") {
		return nil
	}

	objOut := strings.TrimSuffix(c.OutName, filepath.Ext(c.OutName)) + ".o"
	llcArgs := []string{"-filetype=obj"}
	if file.Config != nil && file.Config.Treeshake {
		// Emit per-symbol sections so linker GC can drop unreachable code/data.
		llcArgs = append(llcArgs, "-function-sections", "-data-sections")
	}

	if file.Config != nil {
		triple := buildTargetTriple(file.Config.Arch, file.Config.Vendor, file.Config.Platform)
		llcArgs = append(llcArgs, "--mtriple", triple)
	}

	if c.Ctx != nil {
		llcArgs = append(llcArgs, c.Ctx.StringSlice("llc-args")...)
	}

	llcArgs = append(llcArgs, "-o", objOut, c.OutName)

	cmd := exec.Command("llc", llcArgs...)

	return cmd
}

// buildTargetTriple constructs an LLVM target triple from architecture, vendor, and platform.
func buildTargetTriple(arch, vendor, platform string) string {
	if platform == "" {
		return ""
	}

	// Bare-metal / freestanding targets
	if platform == "none" || platform == "elf" {
		if vendor == "" {
			vendor = "unknown"
		}
		switch arch {
		case "i386", "i686":
			return "i386-unknown-none-elf"
		case "x86_64", "amd64":
			return "x86_64-unknown-none-elf"
		case "arm64", "aarch64":
			return "aarch64-unknown-none-elf"
		case "arm":
			return "arm-unknown-none-elf"
		case "riscv64":
			return "riscv64-unknown-none-elf"
		default:
			return arch + "-" + vendor + "-none-elf"
		}
	}

	// Standard OS targets
	if vendor != "" {
		if arch == "arm64" && platform == "darwin" {
			return "arm64-apple-darwin21.4.0"
		}
		return arch + "-" + vendor + "-" + platform
	}
	if arch == "arm64" && platform == "darwin" {
		return "arm64-apple-darwin21.4.0"
	}
	return arch + "-unknown-" + platform
}
