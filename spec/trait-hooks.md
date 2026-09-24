# Trait Hooks

Trait hooks connect compiler features to user-defined traits. The compiler provides *capabilities*, developers wire them up.

## Hook Attributes (Implemented)

```gecko
// Cleanup hook - compiler calls .drop() when value goes out of scope
@drop_hook(.drop)
trait Drop {
    func drop(self): void
}
```

## Borrow Hooks

```gecko
@borrow_hook(.borrow)
public trait Borrow<R> { func borrow(self): R }

@borrow_mut_hook(.borrow_mut)
public trait BorrowMut<R> { func borrow_mut(self): R }
```

`@borrow(owner)` and `@borrow_mut(owner)` invoke these hooks on a named binding.
The method names and return types come from the registered traits. The stdlib
returns `Ref<T>` and `RefMut<T>` from `BorrowCell<T>`; custom implementations can
supply their own view types. The hooks do not confer raw memory permission.
See [Memory Model](memory.md) for runtime exclusivity and current limits.

## Hook Attributes (Planned, Not Yet Implemented)

```gecko
// Planned: compiler-guided implicit copy
@copy_hook(.copy)
trait Copy {
    func copy(self): Self
}

// Planned: compiler-guided explicit clone
@clone_hook(.clone)  
trait Clone {
    func clone(self): Self
}
```

## Operator Hooks

```gecko
// Arithmetic
@add_hook(.add)      trait Add<T> { func add(self, other: T): T }
@sub_hook(.sub)      trait Sub<T> { func sub(self, other: T): T }
@mul_hook(.mul)      trait Mul<T> { func mul(self, other: T): T }
@div_hook(.div)      trait Div<T> { func div(self, other: T): T }
@neg_hook(.neg)      trait Neg { func neg(self): Self }
@not_hook(.not)      trait Not { func not(self): Self }

// Comparison
@eq_hook(.eq)        trait Eq<T> { func eq(self, other: T): bool }
@ne_hook(.ne)        trait Ne<T> { func ne(self, other: T): bool }
@lt_hook(.lt)        trait Lt<T> { func lt(self, other: T): bool }
@gt_hook(.gt)        trait Gt<T> { func gt(self, other: T): bool }
@le_hook(.le)        trait Le<T> { func le(self, other: T): bool }
@ge_hook(.ge)        trait Ge<T> { func ge(self, other: T): bool }

// Bitwise
@bitand_hook(.bitand)  trait BitAnd<T> { func bitand(self, other: T): T }
@bitor_hook(.bitor)    trait BitOr<T> { func bitor(self, other: T): T }
@bitxor_hook(.bitxor)  trait BitXor<T> { func bitxor(self, other: T): T }
@shl_hook(.shl)        trait Shl<T> { func shl(self, other: T): T }
@shr_hook(.shr)        trait Shr<T> { func shr(self, other: T): T }
```

## Indexing Hooks

```gecko
// arr[i] read access
@index_hook(.get)
trait Index<I, T> {
    func get(self, index: I): T
}

// arr[i] = val write access
@index_mut_hook(.set)
trait IndexMut<I, T> {
    func set(self, index: I, value: T): void
}
```

## Iterator Hooks

```gecko
// for-loop desugaring
@iterator_hook(.next, .has_next)
trait Iterator<T> {
    func next(self): T
    func has_next(self): bool
}

// for x in collection { } desugaring
@into_iterator_hook(.iter)
trait IntoIterator<T> {
    func iter(self): Iterator<T>
}
```

## Hook Rules

1. **One hook per capability** - Only one trait can be registered per hook type
2. **Signature verification** - Compiler verifies trait matches expected signature
3. **No hook = no sugar** - If `@add_hook` isn't defined, `+` only works for primitives
4. **Lookup order** - Resolve hooks from local module/imports first; if stdlib is present, std hooks may be used as fallback
5. **Missing hook behavior** - Hook-dependent features (for example `try`, `or`, trait-based indexing/iteration) are compile errors when no hook is found

## Pointer and Nullability Operations

Pointer/nullability operations are not modeled as privileged traits.
Use explicit pointer syntax with canonical dereference:

```gecko
@unsafe { let value = @deref(ptr) }
let is_null = ptr == null
let next = (ptr as uint64 + 1) as int*
```

Do not rely on pseudo-traits like `Pointer` or `NonNullable`.

## Example: Complete Drop Implementation

```gecko
package mymodule

// Define the trait with the hook
@drop_hook(.drop)
public trait Drop {
    func drop(self): void
}

// Implement for a type
public class FileHandle {
    let fd: int32
}

impl Drop for FileHandle {
    func drop(self): void {
        // Close the file descriptor
        close(self.fd)
    }
}

// Usage - compiler inserts .drop() call at scope exit
func process_file(): void {
    let f: FileHandle = open_file("data.txt")
    // ... use f ...
}   // <- compiler calls f.drop() here
```

## Opting Out

Pass `--no-auto-drop` to compile/build/run to disable inserted cleanup while
retaining explicit drops and defers. Projects can also avoid defining a Drop hook:

```gecko
// Kernel code - no automatic cleanup
package kernel

trait Cleanup {
    func cleanup(self): void
}

// Must call .cleanup() manually - no compiler magic
impl Cleanup for Buffer {
    func cleanup(self): void {
        kfree(self.ptr)
    }
}
```

See [Traits](traits.md) for declarations, implementations, and coherence rules.
