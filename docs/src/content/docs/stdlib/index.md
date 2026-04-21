---
title: Standard Library
description: Gecko standard library reference
---

The standard library provides memory management primitives for Gecko programs.

## Packages

### collections


### traits

- [**Drop** (trait)](/stdlib/traits-drop/) - Drop - Trait for types that need cleanup when going out of scope.
- [**Clone** (trait)](/stdlib/traits-clone/) - Clone - Trait for types that can be explicitly duplicated.
- [**Copy** (trait)](/stdlib/traits-copy/) - Copy - Marker trait for types that are safe to copy bitwise.
- [**Default** (trait)](/stdlib/traits-default/) - Default - Trait for types with a default value.
- [**From** (trait)](/stdlib/traits-from/) - From<T> - Trait for converting from type T.
- [**Into** (trait)](/stdlib/traits-into/) - Into<T> - Trait for converting into type T.
- [**TryFrom** (trait)](/stdlib/traits-tryfrom/) - TryFrom<T> - Fallible conversion from type T.
- [**Iterator** (trait)](/stdlib/traits-iterator/) - Iterator<T> - Trait for types that produce a sequence of values.
- [**IntoIterator** (trait)](/stdlib/traits-intoiterator/) - IntoIterator<T> - Trait for types convertible to an iterator.
- [**Index** (trait)](/stdlib/traits-index/) - Index<I, T> - Trait for read indexing (a[i]).
- [**IndexMut** (trait)](/stdlib/traits-indexmut/) - IndexMut<I, T> - Trait for write indexing (a[i] = v).
- [**Sized** (trait)](/stdlib/traits-sized/) - Sized - Trait for types with known size at compile time.
- [**Hash** (trait)](/stdlib/traits-hash/) - Hash - Trait for types that can be hashed.
- [**Debug** (trait)](/stdlib/traits-debug/) - Debug - Trait for debug formatting.
- [**Display** (trait)](/stdlib/traits-display/) - Display - Trait for user-facing formatting.
- [**FnOnce** (trait)](/stdlib/traits-fnonce/) - FnOnce<A, R> - Callable types consumed on call.
- [**Fn** (trait)](/stdlib/traits-fn/) - Fn<A, R> - Callable types (multiple calls allowed).

### memory


### rc

- [**RcInner**](/stdlib/rc-rcinner/) - Internal structure holding reference counts and value.
- [**Rc**](/stdlib/rc-rc/) - Rc<T> - Reference Counted Smart Pointer.

### vec

- [**Vec**](/stdlib/vec-vec/) - Vec<T> - A growable, heap-allocated array.

### box

- [**Box**](/stdlib/box-box/) - Box<T> - Unique ownership smart pointer.

### weak

- [**Weak**](/stdlib/weak-weak/) - Weak<T> - Non-owning reference to `Rc<T>` data.

### option

- [**Option**](/stdlib/option-option/) - Option<T> - Represents an optional value.

### slice

- [**Slice**](/stdlib/slice-slice/) - 

### string

- [**StringIterator**](/stdlib/string-stringiterator/) - StringIterator - Iterator over bytes in a String.
- [**String**](/stdlib/string-string/) - String - A growable, heap-allocated string.
- [**StringBuilder**](/stdlib/string-stringbuilder/) - StringBuilder - A simple growable byte buffer for building strings.
- [**Add** (trait)](/stdlib/string-add/) - Add<T> - Trait for the `+` operator.

### raw

- [**Raw**](/stdlib/raw-raw/) - Raw<T> - Unsafe pointer wrapper for low-level memory operations.

### core


### ops

- [**Add** (trait)](/stdlib/ops-add/) - Add<T> - Trait for the `+` operator.
- [**Sub** (trait)](/stdlib/ops-sub/) - Sub<T> - Trait for the `-` operator.
- [**Mul** (trait)](/stdlib/ops-mul/) - Mul<T> - Trait for the `*` operator.
- [**Div** (trait)](/stdlib/ops-div/) - Div<T> - Trait for the `/` operator.
- [**Neg** (trait)](/stdlib/ops-neg/) - Neg - Trait for unary `-` operator.
- [**Not** (trait)](/stdlib/ops-not/) - Not - Trait for unary `!` operator.
- [**BitAnd** (trait)](/stdlib/ops-bitand/) - BitAnd<T> - Trait for the `&` operator.
- [**BitOr** (trait)](/stdlib/ops-bitor/) - BitOr<T> - Trait for the `|` operator.
- [**BitXor** (trait)](/stdlib/ops-bitxor/) - BitXor<T> - Trait for the `^` operator.
- [**Shl** (trait)](/stdlib/ops-shl/) - Shl<T> - Trait for the `<<` operator.
- [**Shr** (trait)](/stdlib/ops-shr/) - Shr<T> - Trait for the `>>` operator.
- [**Eq** (trait)](/stdlib/ops-eq/) - Eq<T> - Trait for the `==` operator.
- [**Ne** (trait)](/stdlib/ops-ne/) - Ne<T> - Trait for the `!=` operator.
- [**Lt** (trait)](/stdlib/ops-lt/) - Lt<T> - Trait for the `<` operator.
- [**Gt** (trait)](/stdlib/ops-gt/) - Gt<T> - Trait for the `>` operator.
- [**Le** (trait)](/stdlib/ops-le/) - Le<T> - Trait for the `<=` operator.
- [**Ge** (trait)](/stdlib/ops-ge/) - Ge<T> - Trait for the `>=` operator.

### std


