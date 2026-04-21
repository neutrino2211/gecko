# Documentation TODO

Ordered by priority. Check off items as completed.

## High Priority (Core Learning Path)

- [x] **1. Fix README.md roadmap** - Update to reflect actual feature status
- [x] **2. Generics documentation** - Type parameters, constraints, multiple constraints
- [x] **3. Module system & imports** - import syntax, selective imports, directory imports
- [x] **4. Hooks & operator overloading** - All hook attributes, how to implement operators
- [x] **5. For-in loops** - Iterator protocol, @iterator_hook usage

## Medium Priority (Complete the Picture)

- [x] **6. Enums** - Declaration, usage, pattern matching
- [x] **7. Visibility** - public/private, module boundaries
- [x] **8. Intrinsics reference** - @deref, @size_of, @is_null, etc.
- [x] **9. CLI/tooling reference** - All commands and flags
- [x] **10. gecko.toml guide** - Project configuration, dependencies, build profiles

## Lower Priority (Advanced Users)

- [x] **11. Cross-compilation guide** - Target flags, freestanding builds
- [x] **12. Kernel development guide** - @packed, @section, @naked, volatile, inline asm
- [x] **13. FFI patterns** - cimport, external declarations, C struct mapping

## Stdlib Documentation Fixes

- [x] **14. Fix core/ops section** - Fixed docgen to include traits in index
- [x] **15. Fix core/traits section** - Fixed docgen to include traits in index
- [ ] **16. Add usage examples to stdlib pages** - Vec, String, Box, etc.

## Syntax Highlighting

- [x] **17. Shiki grammar for Gecko** - Created gecko.tmLanguage.json
- [x] **18. Astro integration** - Configured expressiveCode with custom language

---

## Notes

- CLAUDE.md is the current source of truth for language features
- Docs website uses Starlight (Astro) - files in `docs/src/content/docs/`
- Stdlib docs are auto-generated via `gecko doc` command
- Examples in `examples/` and `test_sources/compile_tests/` serve as reference
