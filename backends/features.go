package backends

// Feature represents a language feature that may or may not be supported by a backend
type Feature string

const (
	// Core features - minimal set for any backend
	FeatureFunctions    Feature = "functions"
	FeaturePrimitives   Feature = "primitives"    // int, bool, uint8, etc.
	FeatureControlFlow  Feature = "control_flow"  // if, else, while, for
	FeatureBasicOps     Feature = "basic_ops"     // arithmetic, comparison, logical
	FeatureVariables    Feature = "variables"     // let, const

	// Structured features - require memory layout
	FeatureClasses      Feature = "classes"       // class definitions
	FeatureStructs      Feature = "structs"       // struct literals
	FeatureArrays       Feature = "arrays"        // fixed-size arrays
	FeatureStrings      Feature = "strings"       // string type

	// Standard features - higher-level abstractions
	FeatureGenerics     Feature = "generics"      // type parameters
	FeatureTraits       Feature = "traits"        // trait definitions
	FeatureImpl         Feature = "impl"          // trait implementations
	FeatureOperatorOvld Feature = "operator_overload"
	FeatureTypeInfer    Feature = "type_inference"
	FeatureImports      Feature = "imports"       // module imports

	// Low-level features - direct memory access
	FeaturePointers     Feature = "pointers"      // pointer types, &, *
	FeatureDeref        Feature = "deref"         // @deref intrinsic
	FeatureExternDecl   Feature = "extern_decl"   // declare external
	FeatureCasts        Feature = "casts"         // as keyword
	FeatureVolatile     Feature = "volatile"      // volatile pointers
	FeatureAddressOf    Feature = "address_of"    // & operator

	// Freestanding features - OS/kernel development
	FeatureNaked        Feature = "naked"         // @naked attribute
	FeatureNoReturn     Feature = "noreturn"      // @noreturn attribute
	FeatureSection      Feature = "section"       // @section attribute
	FeaturePacked       Feature = "packed"        // @packed attribute
	FeatureInlineAsm    Feature = "inline_asm"    // asm {} blocks
)

// FeatureSet defines which features a backend supports
type FeatureSet struct {
	supported  map[Feature]bool
	name       string
	toolchain  string   // e.g., "native", "wasm", "jit"
	compatible []string // list of compatible backend names
}

// NewFeatureSet creates a new feature set for a backend
func NewFeatureSet(name string) *FeatureSet {
	return &FeatureSet{
		supported: make(map[Feature]bool),
		name:      name,
	}
}

// Enable enables a feature
func (fs *FeatureSet) Enable(features ...Feature) *FeatureSet {
	for _, f := range features {
		fs.supported[f] = true
	}
	return fs
}

// Disable disables a feature
func (fs *FeatureSet) Disable(features ...Feature) *FeatureSet {
	for _, f := range features {
		fs.supported[f] = false
	}
	return fs
}

// Supports checks if a feature is supported
func (fs *FeatureSet) Supports(f Feature) bool {
	return fs.supported[f]
}

// SupportsString checks if a feature is supported (string version for interface compatibility)
func (fs *FeatureSet) SupportsString(f string) bool {
	return fs.supported[Feature(f)]
}

// GetUnsupported returns a list of unsupported features from the given list
func (fs *FeatureSet) GetUnsupported(features []Feature) []Feature {
	var unsupported []Feature
	for _, f := range features {
		if !fs.supported[f] {
			unsupported = append(unsupported, f)
		}
	}
	return unsupported
}

// Name returns the backend name
func (fs *FeatureSet) Name() string {
	return fs.name
}

// Toolchain returns the toolchain identifier (e.g., "native", "wasm")
func (fs *FeatureSet) Toolchain() string {
	return fs.toolchain
}

// SetToolchain sets the toolchain and compatible backends
func (fs *FeatureSet) SetToolchain(toolchain string, compatible ...string) *FeatureSet {
	fs.toolchain = toolchain
	fs.compatible = compatible
	return fs
}

// CanImportFrom checks if this backend can import from another backend
func (fs *FeatureSet) CanImportFrom(other string) bool {
	// Same backend is always compatible
	if fs.name == other {
		return true
	}
	// Check explicit compatibility list
	for _, c := range fs.compatible {
		if c == other {
			return true
		}
	}
	return false
}

// CanImportFromToolchain checks if this backend can import from a toolchain
func (fs *FeatureSet) CanImportFromToolchain(otherToolchain string) bool {
	return fs.toolchain == otherToolchain
}

// Predefined feature groups for convenience

// CoreFeatures returns the minimal feature set
func CoreFeatures() []Feature {
	return []Feature{
		FeatureFunctions,
		FeaturePrimitives,
		FeatureControlFlow,
		FeatureBasicOps,
		FeatureVariables,
	}
}

// StructuredFeatures returns features requiring memory layout
func StructuredFeatures() []Feature {
	return []Feature{
		FeatureClasses,
		FeatureStructs,
		FeatureArrays,
		FeatureStrings,
	}
}

// StandardFeatures returns higher-level language features
func StandardFeatures() []Feature {
	return []Feature{
		FeatureGenerics,
		FeatureTraits,
		FeatureImpl,
		FeatureOperatorOvld,
		FeatureTypeInfer,
		FeatureImports,
	}
}

// LowLevelFeatures returns direct memory access features
func LowLevelFeatures() []Feature {
	return []Feature{
		FeaturePointers,
		FeatureDeref,
		FeatureExternDecl,
		FeatureCasts,
		FeatureVolatile,
		FeatureAddressOf,
	}
}

// FreestandingFeatures returns OS/kernel development features
func FreestandingFeatures() []Feature {
	return []Feature{
		FeatureNaked,
		FeatureNoReturn,
		FeatureSection,
		FeaturePacked,
		FeatureInlineAsm,
	}
}

// AllFeatures returns all available features
func AllFeatures() []Feature {
	var all []Feature
	all = append(all, CoreFeatures()...)
	all = append(all, StructuredFeatures()...)
	all = append(all, StandardFeatures()...)
	all = append(all, LowLevelFeatures()...)
	all = append(all, FreestandingFeatures()...)
	return all
}

// NewFullFeatureSet creates a feature set with all features enabled
func NewFullFeatureSet(name string) *FeatureSet {
	fs := NewFeatureSet(name)
	fs.Enable(AllFeatures()...)
	return fs
}

// NewCoreOnlyFeatureSet creates a feature set with only core features
func NewCoreOnlyFeatureSet(name string) *FeatureSet {
	fs := NewFeatureSet(name)
	fs.Enable(CoreFeatures()...)
	return fs
}

// Toolchain identifiers
const (
	ToolchainNative = "native" // GCC/LLVM/system linker
	ToolchainWasm   = "wasm"   // WebAssembly (future)
	ToolchainJIT    = "jit"    // JIT compilation (future)
)

// NewAsmFeatureSet creates a feature set suitable for ASM backend
func NewAsmFeatureSet() *FeatureSet {
	fs := NewFeatureSet("asm")
	fs.Enable(CoreFeatures()...)
	fs.Enable(FreestandingFeatures()...)
	fs.SetToolchain(ToolchainNative, "c", "llvm") // ASM can link with C and LLVM
	return fs
}

// NewCFeatureSet creates a feature set for C backend
func NewCFeatureSet() *FeatureSet {
	fs := NewFullFeatureSet("c")
	fs.SetToolchain(ToolchainNative, "llvm", "asm") // C can link with LLVM and ASM
	return fs
}

// NewLLVMFeatureSet creates a feature set for LLVM backend
func NewLLVMFeatureSet() *FeatureSet {
	fs := NewFullFeatureSet("llvm")
	fs.SetToolchain(ToolchainNative, "c", "asm") // LLVM can link with C and ASM
	return fs
}
