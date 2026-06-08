// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package tests

import (
	"sort"
	"testing"

	"github.com/neutrino2211/gecko/backends"
	"github.com/neutrino2211/gecko/parser"
)

func unsupportedForCode(t *testing.T, code string) []backends.Feature {
	t.Helper()

	file, err := parser.Parser.ParseString("llvm_lite_test.gecko", code)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	used := backends.DetectFeatures(file)
	unsupported := backends.NewLLVMFeatureSet().GetUnsupported(used)
	sort.Slice(unsupported, func(i, j int) bool { return unsupported[i] < unsupported[j] })
	return unsupported
}

func hasFeature(features []backends.Feature, want backends.Feature) bool {
	for _, f := range features {
		if f == want {
			return true
		}
	}
	return false
}

func TestLLVMLiteSupportedSubset(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "core_flow_and_ops",
			code: `package main
func main(): int32 {
    let x: int32 = 1
    let y: int32 = 2
    if x < y {
        return x + y
    }
    return 0
}`,
		},
		{
			name: "classes_structs_arrays",
			code: `package main
class Point {
    let x: int32
    let y: int32
}
func main(): int32 {
    let p: Point = Point { x: 10, y: 25 }
    let a: [4]int32
    a[0] = p.x
    return a[0] + p.y
}`,
		},
		{
			name: "extern_pointers_casts",
			code: `package main
declare external func puts(s: string): int32
func main(): int32 {
    let x: int32 = 7
    let p: int32* = &x
    let raw: uint64 = p as uint64
    let p2: int32* = raw as int32*
    return puts("ok")
}`,
		},
		{
			name: "freestanding_attributes_and_asm",
			code: `package main
@section(".text.boot")
@noreturn
func halt(): void {
    asm { "hlt" }
    while true {}
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsupported := unsupportedForCode(t, tc.code)
			if len(unsupported) > 0 {
				t.Fatalf("expected LLVM-lite supported subset, got unsupported features: %v", unsupported)
			}
		})
	}
}

func TestLLVMLiteUnsupportedFeatures(t *testing.T) {
	tests := []struct {
		name string
		code string
		want backends.Feature
	}{
		{
			name: "volatile_pointers",
			code: `package main
func main(): int32 {
    let p: int32 volatile* = 0 as int32 volatile*
    return 0
}`,
			want: backends.FeatureVolatile,
		},
		{
			name: "deref_intrinsic",
			code: `package main
func main(): int32 {
    let p: int32* = 0 as int32*
    return @deref(p)
}`,
			want: backends.FeatureDeref,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsupported := unsupportedForCode(t, tc.code)
			if !hasFeature(unsupported, tc.want) {
				t.Fatalf("expected unsupported feature %q, got %v", tc.want, unsupported)
			}
		})
	}
}
