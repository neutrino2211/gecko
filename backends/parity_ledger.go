// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

// LoweringOwnership specifies where a feature's implementation responsibility lives.
type LoweringOwnership string

const (
	OwnershipSharedLowering LoweringOwnership = "shared_lowering"
	OwnershipBackendEmitter LoweringOwnership = "backend_emitter"
)

// ParityStatus tracks backend parity state per feature.
type ParityStatus string

const (
	ParitySupported   ParityStatus = "supported"
	ParityInProgress  ParityStatus = "in_progress"
	ParityUnsupported ParityStatus = "unsupported"
)

// FeatureParityRecord is the backend parity ledger entry for one feature.
type FeatureParityRecord struct {
	Feature   Feature
	Owner     LoweringOwnership
	C         ParityStatus
	TestLinks []string
}

// FeatureParityLedger maps every language feature to ownership and parity status.
var FeatureParityLedger = map[Feature]FeatureParityRecord{
	FeatureFunctions:   {Feature: FeatureFunctions, Owner: OwnershipSharedLowering, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeaturePrimitives:  {Feature: FeaturePrimitives, Owner: OwnershipSharedLowering, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureControlFlow: {Feature: FeatureControlFlow, Owner: OwnershipSharedLowering, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureLoopControl: {Feature: FeatureLoopControl, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"test_sources/compile_tests/loops/break_continue.gecko", "tests/compiler_test.go"}},
	FeatureBasicOps:    {Feature: FeatureBasicOps, Owner: OwnershipSharedLowering, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureVariables:   {Feature: FeatureVariables, Owner: OwnershipSharedLowering, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},

	FeatureClasses: {Feature: FeatureClasses, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureStructs: {Feature: FeatureStructs, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureArrays:  {Feature: FeatureArrays, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureStrings: {Feature: FeatureStrings, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},

	FeatureGenerics:     {Feature: FeatureGenerics, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"test_sources/compile_tests/generics/containers.gecko", "test_sources/compile_tests/nested_generics/main.gecko", "tests/compiler_test.go"}},
	FeatureTraits:       {Feature: FeatureTraits, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"test_sources/compile_tests/traits/basic.gecko", "test_sources/compile_tests/trait_inheritance/main.gecko"}},
	FeatureImpl:         {Feature: FeatureImpl, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"test_sources/compile_tests/inherent_impl/main.gecko", "test_sources/compile_tests/coherence/trait_impl_local_trait_foreign_type_ok.gecko"}},
	FeatureOperatorOvld: {Feature: FeatureOperatorOvld, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"test_sources/compile_tests/operator_overload/main.gecko", "test_sources/compile_tests/hooks/operators_arithmetic.gecko"}},
	FeatureTypeInfer:    {Feature: FeatureTypeInfer, Owner: OwnershipSharedLowering, C: ParitySupported, TestLinks: []string{"test_sources/compile_tests/type_inference/main.gecko"}},
	FeatureImports:      {Feature: FeatureImports, Owner: OwnershipSharedLowering, C: ParitySupported, TestLinks: []string{"test_sources/compile_tests/imports/main.gecko"}},

	FeaturePointers:   {Feature: FeaturePointers, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureDeref:      {Feature: FeatureDeref, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"test_sources/compile_tests/intrinsics/basic.gecko"}},
	FeatureIntrinsics: {Feature: FeatureIntrinsics, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"test_sources/compile_tests/intrinsics/basic.gecko"}},
	FeatureExternDecl: {Feature: FeatureExternDecl, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureCasts:      {Feature: FeatureCasts, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureVolatile:   {Feature: FeatureVolatile, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"test_sources/compile_tests/volatile/volatile_pointer.gecko"}},
	FeatureAddressOf:  {Feature: FeatureAddressOf, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},

	FeatureNaked:     {Feature: FeatureNaked, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureNoReturn:  {Feature: FeatureNoReturn, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureSection:   {Feature: FeatureSection, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeaturePacked:    {Feature: FeaturePacked, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
	FeatureInlineAsm: {Feature: FeatureInlineAsm, Owner: OwnershipBackendEmitter, C: ParitySupported, TestLinks: []string{"tests/compiler_test.go"}},
}

func FeatureOwnership(feature Feature) LoweringOwnership {
	record, ok := FeatureParityLedger[feature]
	if !ok {
		return OwnershipBackendEmitter
	}
	return record.Owner
}
