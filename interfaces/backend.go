package interfaces

import (
	"os/exec"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
	"github.com/urfave/cli/v2"
)

// FeatureChecker is the interface for checking backend feature support
type FeatureChecker interface {
	SupportsString(feature string) bool
	Name() string
	Toolchain() string
	CanImportFrom(backend string) bool
}

type BackendInteface interface {
	Init()
	Compile(*BackendConfig) *exec.Cmd
	GetImpls() BackendCodegenImplementations
	ProcessEntries([]*tokens.Entry, *ast.Ast)
	// Features returns the feature set supported by this backend
	Features() FeatureChecker
}

// LazyTypeResolverFunc resolves types from directory imports on demand.
// Returns the parsed file containing the type if found, along with a success flag.
type LazyTypeResolverFunc func(typeName string) (*tokens.File, bool)

// LazyMethodResolverFunc resolves methods from directory imports on demand.
// Returns the parsed file containing the method if found, along with a success flag.
type LazyMethodResolverFunc func(methodName string) (*tokens.File, bool)

// LazyModuleTypeResolverFunc resolves types from a specific module (directory import).
// Used for module-qualified types like shapes.Circle.
type LazyModuleTypeResolverFunc func(moduleName string, typeName string) (*tokens.File, bool)

// TypeSuggestionFunc returns import suggestions for an unresolved type.
type TypeSuggestionFunc func(typeName string) string

type BackendConfig struct {
	File                   string
	OutName                string
	Ctx                    *cli.Context
	SourceFile             *tokens.File
	LazyTypeResolver       LazyTypeResolverFunc       // Resolves types from directory imports
	LazyMethodResolver     LazyMethodResolverFunc     // Resolves methods from directory imports
	LazyModuleTypeResolver LazyModuleTypeResolverFunc // Resolves types from specific module
	SuggestionProvider     TypeSuggestionFunc         // Returns import suggestions for unresolved types
}

type BackendCodegenImplementations interface {
	NewReturn(*ast.Ast)
	NewReturnLiteral(*ast.Ast, *tokens.Expression)

	FuncCall(*ast.Ast, *tokens.FuncCall)
	MethodCall(*ast.Ast, *tokens.MethodCall)
	Declaration(*ast.Ast, *tokens.Declaration)

	NewVariable(*ast.Ast, *tokens.Field)
	NewMethod(*ast.Ast, *tokens.Method)

	ParseExpression(*ast.Ast, *tokens.Expression)

	ProcessEntries(*ast.Ast, []*tokens.Entry)

	NewClass(*ast.Ast, *tokens.Class)
	NewDeclaration(*ast.Ast, *tokens.Declaration)
	NewImplementation(*ast.Ast, *tokens.Implementation)
	NewTrait(*ast.Ast, *tokens.Trait)
	NewEnum(*ast.Ast, *tokens.Enum)

	// Control flow
	NewIf(*ast.Ast, *tokens.If)
	NewLoop(*ast.Ast, *tokens.Loop)
	NewAssignment(*ast.Ast, *tokens.Assignment)
	NewBreak(*ast.Ast)
	NewContinue(*ast.Ast)

	// Inline assembly
	NewAsm(*ast.Ast, *tokens.Asm)

	// Intrinsic statements
	IntrinsicStatement(*ast.Ast, *tokens.Intrinsic)

	// C imports
	NewCImport(*ast.Ast, *tokens.CImport)
}
