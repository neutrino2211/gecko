// spec: spec/types.md, spec/traits.md, spec/modules.md, spec/scoping.md

package tests

type compileTest struct {
	name         string
	file         string
	expectedExit int
	shouldFail   bool
}

type compileOnlyTest struct {
	name string
	file string
}

var allTestBackends = []string{"c"}

var compileTests = []compileTest{
	{"move_intrinsic_error", "test_sources/compile_tests/move_intrinsic_error/main.gecko", 0, true},
	{"borrow_custom_hook", "test_sources/compile_tests/borrow_custom_hook/main.gecko", 0, false},
	{"borrow_const_error", "test_sources/compile_tests/borrow_const_error/main.gecko", 0, true},
	{"borrow_raw_constructor_error", "test_sources/compile_tests/borrow_raw_constructor_error/main.gecko", 0, true},
	{"borrow_temporary_error", "test_sources/compile_tests/borrow_temporary_error/main.gecko", 0, true},
	{"borrow_move_error", "test_sources/compile_tests/borrow_move_error/main.gecko", 0, true},
	{"unsafe_handler_exit", "test_sources/compile_tests/unsafe_handler_exit/main.gecko", 0, false},
	{"memory_automation", "test_sources/compile_tests/memory_automation/main.gecko", 4, false},
	{"typed_borrows", "test_sources/compile_tests/typed_borrows/main.gecko", 0, false},
	{"raw_drop_context_error", "test_sources/compile_tests/raw_drop_context_error/main.gecko", 0, true},
	{"raw_generic_read_error", "test_sources/compile_tests/raw_generic_read_error/main.gecko", 0, true},
	{"cleanup_paths", "test_sources/compile_tests/cleanup_paths/main.gecko", 0, false},
	{"memory_lifecycle", "test_sources/compile_tests/memory_lifecycle/main.gecko", 0, false},
	{"owned_aggregates", "test_sources/compile_tests/owned_aggregates/main.gecko", 0, false},
	{"box_unchecked_error", "test_sources/compile_tests/box_unchecked_error/main.gecko", 0, true},
	{"implicit_arg_move_error", "test_sources/compile_tests/implicit_arg_move_error/main.gecko", 0, true},
	{"raw_access", "test_sources/compile_tests/raw_access/main.gecko", 0, false},
	{"raw_deref_read_error", "test_sources/compile_tests/raw_deref_read_error/main.gecko", 0, true},
	{"raw_deref_store_error", "test_sources/compile_tests/raw_deref_store_error/main.gecko", 0, true},
	{"raw_pointer_index_read_error", "test_sources/compile_tests/raw_pointer_index_read_error/main.gecko", 0, true},
	{"raw_pointer_index_store_error", "test_sources/compile_tests/raw_pointer_index_store_error/main.gecko", 0, true},
	{"raw_pointer_method_read_error", "test_sources/compile_tests/raw_pointer_method_read_error/main.gecko", 0, true},
	{"raw_pointer_field_read_error", "test_sources/compile_tests/raw_pointer_field_read_error/main.gecko", 0, true},
	{"raw_pointer_field_store_error", "test_sources/compile_tests/raw_pointer_field_store_error/main.gecko", 0, true},

	{"string_storage_refactor", "test_sources/compile_tests/string_storage_refactor/main.gecko", 0, false},
	{"unsafe_setup", "test_sources/compile_tests/unsafe_setup/main.gecko", 0, false},
	{"unsafe_setup_failure", "test_sources/compile_tests/unsafe_setup_failure/main.gecko", 0, false},
	{"unsafe_setup_order", "test_sources/compile_tests/unsafe_setup_order/main.gecko", 0, false},
	{"unsafe_setup_pointer", "test_sources/compile_tests/unsafe_setup_pointer/main.gecko", 0, false},
	// Trait tests
	{"traits_basic", "test_sources/compile_tests/traits/basic.gecko", 30, false},
	{"traits_constraints", "test_sources/compile_tests/traits/constraints.gecko", 42, false},

	// Generic tests
	{"generics_containers", "test_sources/compile_tests/generics/containers.gecko", 19, false},

	// Pointer tests
	{"pointers_nonnull", "test_sources/compile_tests/pointers/nonnull.gecko", 42, false},

	// External type tests
	{"external_types_basic", "test_sources/compile_tests/external_types/basic.gecko", 55, false},

	// Intrinsics tests
	{"intrinsics_basic", "test_sources/compile_tests/intrinsics/basic.gecko", 17, false},

	// Unsafe handler tests
	{"unsafe_handler_catch", "test_sources/compile_tests/unsafe_handler_catch/main.gecko", 0, false},
	{"unsafe_handler_nonnull", "test_sources/compile_tests/unsafe_handler_nonnull/main.gecko", 42, false},
	{"unsafe_handler_result_expr", "test_sources/compile_tests/unsafe_handler_result_expr/main.gecko", 99, false},
	{"unsafe_handler_result_err_expr", "test_sources/compile_tests/unsafe_handler_result_err_expr/main.gecko", 1, false},
	{"unsafe_handler_result_reassign", "test_sources/compile_tests/unsafe_handler_result_reassign/main.gecko", 20, false},
	{"unsafe_handler_group_expr", "test_sources/compile_tests/unsafe_handler_group_expr/main.gecko", 1, false},

	// Builtin traits tests
	{"builtin_traits_pointer", "test_sources/compile_tests/builtin_traits/pointer.gecko", 43, false},
	{"builtin_traits_nonnull", "test_sources/compile_tests/builtin_traits/nonnull.gecko", 40, false},

	// String module tests
	{"string_builder", "test_sources/compile_tests/string_builder/main.gecko", 0, false},
	{"backtick_multiline_string", "test_sources/compile_tests/strings/backtick_multiline.gecko", 0, false},

	// Raw pointer tests
	{"raw_pointer", "test_sources/compile_tests/raw_pointer/main.gecko", 0, false},

	// Static method tests
	{"static_methods", "test_sources/compile_tests/static_methods/main.gecko", 0, false},
	{"external_static_method", "test_sources/compile_tests/external_static_method/main.gecko", 42, false},

	// Memory types
	{"box_type", "test_sources/compile_tests/box_type/main.gecko", 0, false},
	{"rc_type", "test_sources/compile_tests/rc_type/main.gecko", 0, false},
	{"weak_type", "test_sources/compile_tests/weak_type/main.gecko", 0, false},
	{"buffer_type", "test_sources/compile_tests/buffer_type/main.gecko", 42, false},

	// Type inference
	{"type_inference", "test_sources/compile_tests/type_inference/main.gecko", 0, false},
	{"type_inference_advanced", "test_sources/compile_tests/type_inference_advanced/main.gecko", 122, false},

	// Stdlib types
	{"stdlib_string", "test_sources/compile_tests/stdlib_string/main.gecko", 0, false},
	{"stdlib_vec", "test_sources/compile_tests/stdlib_vec/main.gecko", 0, false},
	{"stdlib_vec_struct", "test_sources/compile_tests/stdlib_vec_struct/main.gecko", 42, false},
	{"stdlib_option", "test_sources/compile_tests/stdlib_option/main.gecko", 0, false},
	{"null_literal", "test_sources/compile_tests/null_literal/main.gecko", 42, false},

	// Operator overloading
	{"operator_overload", "test_sources/compile_tests/operator_overload/main.gecko", 0, false},

	// Logical operators
	{"logical_ops", "test_sources/compile_tests/logical_ops/main.gecko", 0, false},

	// Freestanding traits
	{"freestanding_traits", "test_sources/compile_tests/freestanding_traits/main.gecko", 0, false},

	// Inherent implementations
	{"inherent_impl", "test_sources/compile_tests/inherent_impl/main.gecko", 35, false},
	{"coherence_local_trait_foreign_type_ok", "test_sources/compile_tests/coherence/trait_impl_local_trait_foreign_type_ok.gecko", 35, false},
	{"coherence_foreign_trait_local_type_ok", "test_sources/compile_tests/coherence/trait_impl_foreign_trait_local_type_ok.gecko", 21, false},

	// Directory imports with lazy resolution
	{"directory_imports", "test_sources/compile_tests/directory_imports/main.gecko", 35, false},

	// Qualified type syntax (module.Type)
	{"qualified_types", "test_sources/compile_tests/qualified_types/main.gecko", 75, false},

	// Hooks
	{"hooks_drop", "test_sources/compile_tests/hooks/drop_hook.gecko", 42, false},
	{"hooks_drop_return_field", "test_sources/compile_tests/hooks/drop_hook_return_field.gecko", 43, false},
	{"hooks_operator_add", "test_sources/compile_tests/hooks/operator_add.gecko", 42, false},
	{"hooks_operators_arithmetic", "test_sources/compile_tests/hooks/operators_arithmetic.gecko", 44, false},
	{"hooks_operators_comparison", "test_sources/compile_tests/hooks/operators_comparison.gecko", 63, false},
	{"hooks_operators_bitwise", "test_sources/compile_tests/hooks/operators_bitwise.gecko", 79, false},
	{"hooks_operators_unary", "test_sources/compile_tests/hooks/operators_unary.gecko", 42, false},
	{"hooks_custom_operator_names", "test_sources/compile_tests/hooks/custom_operator_hooks.gecko", 42, false},

	// Examples
	{"example_traits", "examples/traits/shapes.gecko", 93, false},
	{"example_stdlib", "examples/stdlib/demo.gecko", 80, false},
	{"example_c_interop", "examples/c_interop/main.gecko", 105, false},
	{"example_string_builder", "examples/string_builder/demo.gecko", 0, false},

	// Visibility tests
	{"visibility_public_access", "test_sources/compile_tests/visibility/public_access.gecko", 42, false},

	// Index hook tests
	{"index_hook", "test_sources/compile_tests/index_hook/main.gecko", 42, false},

	// Iterator / for-in loop tests
	{"for_in_loop", "test_sources/compile_tests/for_in_loop/main.gecko", 0, false},
	{"for_in_capture", "test_sources/compile_tests/for_in_capture/main.gecko", 3, false},

	// Lazy resolution tests
	{"lazy_method_resolution", "test_sources/compile_tests/lazy_method_resolution/main.gecko", 0, false},

	// Narrowing tests
	{"narrowing_test", "test_sources/compile_tests/narrowing_test/main.gecko", 0, false},

	// Type checking runtime tests
	{"type_checking_default_impl_generic", "test_sources/compile_tests/type_checking/default_impl_generic.gecko", 0, false},
	{"type_checking_default_impl_valid", "test_sources/compile_tests/type_checking/default_impl_valid.gecko", 42, false},
	{"type_checking_generic_valid", "test_sources/compile_tests/type_checking/generic_valid.gecko", 0, false},

	// C import tests
	{"cimport", "test_sources/compile_tests/cimport/main.gecko", 0, false},
	{"import_use_constants", "test_sources/compile_tests/import_use_constants/main.gecko", 42, false},

	// Packed structs
	{"packed", "test_sources/compile_tests/packed/packed.gecko", 0, false},

	// Struct literals
	{"struct_literal", "test_sources/compile_tests/struct_literal/struct_literal.gecko", 0, false},
	{"struct_inline", "test_sources/compile_tests/struct_literal/struct_inline.gecko", 0, false},

	// Fixed-size arrays
	{"fixed_arrays", "test_sources/compile_tests/fixed_arrays/fixed_arrays.gecko", 0, false},

	// Type checking valid code
	{"typecheck_valid", "test_sources/compile_tests/typecheck/typecheck_valid.gecko", 0, false},

	// Enums
	{"enums", "test_sources/compile_tests/enums/main.gecko", 2, false},

	// Nested generics (3+ levels)
	{"nested_generics", "test_sources/compile_tests/nested_generics/main.gecko", 42, false},

	// Generic trait implementations (impl<T> Trait for Class<T>)
	{"generic_trait_impl", "test_sources/compile_tests/generic_trait_impl/main.gecko", 0, false},

	// String iteration
	{"string_iter", "test_sources/compile_tests/string_iter/main.gecko", 0, false},

	// Multiple trait constraints (T is A & B)
	{"multiple_constraints", "test_sources/compile_tests/multiple_constraints/main.gecko", 31, false},

	// Trait inheritance (trait Child: Parent)
	{"trait_inheritance", "test_sources/compile_tests/trait_inheritance/main.gecko", 42, false},
	{"trait_inheritance_transitive", "test_sources/compile_tests/trait_inheritance/transitive.gecko", 42, false},
	{"trait_inheritance_inherited_defaults", "test_sources/compile_tests/trait_inheritance/inherited_defaults.gecko", 30, false},
	{"trait_inheritance_imported_parent", "test_sources/compile_tests/trait_inheritance/imported_parent.gecko", 42, false},

	// Circular dependencies - pointer cycles are allowed
	{"circular_deps_pointer", "test_sources/compile_tests/circular_deps/pointer_cycle.gecko", 0, false},

	// Error handling - try and or expressions
	{"error_handling_or_simple", "test_sources/compile_tests/error_handling_simple/main.gecko", 0, false},
	{"error_handling_or_generic", "test_sources/compile_tests/error_handling_generic/main.gecko", 0, false},
	{"error_handling_or_lazy", "test_sources/compile_tests/error_handling_or_lazy/main.gecko", 0, false},
	{"error_handling_try_generic", "test_sources/compile_tests/error_handling_try_generic/main.gecko", 0, false},
	{"error_handling_try_imported_option", "test_sources/compile_tests/error_handling_try_imported_option/main.gecko", 42, false},
	{"error_handling_try_string", "test_sources/compile_tests/error_handling_try_string/main.gecko", 0, false},
	{"error_handling_try_stdlib", "test_sources/compile_tests/error_handling_try_stdlib/main.gecko", 0, false},
	{"error_handling_try_or_assignment", "test_sources/compile_tests/error_handling_try_or_assignment/main.gecko", 0, false},

	// Runtime-checked stdlib FFI boundary constructors
	{"ffi_runtime_guards", "test_sources/compile_tests/ffi_runtime_guards/main.gecko", 0, false},

	// New language features
	{"compound_assignment", "test_sources/compile_tests/compound_assignment/main.gecko", 6, false},
	{"compound_assignment_bitwise_modulo", "test_sources/compile_tests/compound_assignment/bitwise_modulo.gecko", 0, false},
	{"match_expression", "test_sources/compile_tests/match/main.gecko", 20, false},
	{"match_or_patterns", "test_sources/compile_tests/match/or_patterns.gecko", 30, false},
	{"match_range_patterns", "test_sources/compile_tests/match/range_patterns.gecko", 200, false},
	{"match_guard_patterns", "test_sources/compile_tests/match/guard_patterns.gecko", 100, false},
	{"match_enum", "test_sources/compile_tests/match/enum_match.gecko", 1, false},
	{"match_struct_destructure", "test_sources/compile_tests/match/struct_destructure.gecko", 1, false},
	{"match_comprehensive", "test_sources/compile_tests/match/pattern_comprehensive.gecko", 0, false},
	{"ternary", "test_sources/compile_tests/ternary/main.gecko", 0, false},
	{"incdec", "test_sources/compile_tests/incdec/main.gecko", 0, false},
	{"defer_statement", "test_sources/compile_tests/defer/main.gecko", 42, false},
	{"lambda_expressions", "test_sources/compile_tests/lambda/main.gecko", 16, false},
	{"closure_capture", "test_sources/compile_tests/closure/basic_capture.gecko", 15, false},
	{"closure_capture_assignment", "test_sources/compile_tests/closure/assignment_capture.gecko", 15, false},
	{"self_type", "test_sources/compile_tests/self_type/main.gecko", 0, false},
	{"import_alias", "test_sources/compile_tests/import_alias/main.gecko", 0, false},
	{"destructuring", "test_sources/compile_tests/destructuring/main.gecko", 0, false},
	{"where_clause", "test_sources/compile_tests/where_clause/main.gecko", 0, false},
	{"slice_api", "test_sources/compile_tests/slice/main.gecko", 0, false},
	{"readonly_qualifier", "test_sources/compile_tests/readonly/main.gecko", 0, false},

	// TODO: Fix these tests
	// {"integers", "test_sources/compile_tests/ints/int.gecko", 0, false}, // printf declaration issues
}

var compileOnlyTests = []compileOnlyTest{
	// Backlog directories that are compile-valid but may be platform/runtime dependent.
	{"example_gnote", "examples/GNote/src/main.gecko"},
	{"example_adw_app", "examples/adw_app/src/main.gecko"},
	{"example_hello_kernel", "examples/hello_kernel/main.gecko"},
	{"example_hello_kernel_vga", "examples/hello_kernel/vga.gecko"},
	{"array_index", "test_sources/compile_tests/array_index/array_index.gecko"},
	{"asm", "test_sources/compile_tests/asm/asm.gecko"},
	{"attributes_entry_point", "test_sources/compile_tests/attributes/entry_point.gecko"},
	{"attributes_packed", "test_sources/compile_tests/attributes/packed.gecko"},
	{"casts", "test_sources/compile_tests/casts/cast.gecko"},
	{"comprehensive", "test_sources/compile_tests/comprehensive/test.gecko"},
	{"globals", "test_sources/compile_tests/globals/globals.gecko"},
	{"globals_simple", "test_sources/compile_tests/globals/simple_globals.gecko"},
	{"imports", "test_sources/compile_tests/imports/main.gecko"},
	{"loops_break_continue", "test_sources/compile_tests/loops/break_continue.gecko"},
	{"out_params", "test_sources/compile_tests/out_params/main.gecko"},
	{"cimport_stdlib_redecl", "test_sources/compile_tests/cimport/stdlib_redecl.gecko"},
	{"strings_args", "test_sources/compile_tests/strings/args.gecko"},
	{"strings_greeting", "test_sources/compile_tests/strings/greeting.gecko"},
	{"volatile_pointer", "test_sources/compile_tests/volatile/volatile_pointer.gecko"},
	{"error_handling_try_invalid", "test_sources/compile_tests/error_handling_try_invalid/main.gecko"},
	{"foreign_nullability", "test_sources/compile_tests/foreign_nullability/main.gecko"},
}
