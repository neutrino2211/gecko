# LLVM-Only Example

This example is pinned to LLVM at file scope with:

```gecko
@backend("llvm")
```

## Compile

```bash
gecko compile --ir-only examples/llvm_only/main.gecko
```

## Notes

- The file-level `@backend("llvm")` overrides backend selection for this source file.
- This example stays within the currently supported LLVM-lite subset.
