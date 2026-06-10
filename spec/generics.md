# Generics

Gecko supports generics through monomorphization (compile-time specialization).

## Syntax

Type parameters in angle brackets:

```gecko
func identity<T>(x: T): T {
    return x
}

class Box<T> {
    let value: T
}
```

## Generic Functions

```gecko
func first<T>(a: T, b: T): T {
    return a
}

// Usage - type inferred:
let x: int = 1
let y: int = 2
let z = first(x, y)

// Inference can also use expected type context:
let q: int = first(1, 2)

// Or explicit:
let explicit = first<int>(x, y)
```

## Generic Classes

```gecko
class Option<T> {
    let value: T
    let has_value: bool
    
    func some(val: T): Option<T> {
        let opt: Option<T>
        opt.value = val
        opt.has_value = true
        return opt
    }
    
    func none(): Option<T> {
        let opt: Option<T>
        opt.has_value = false
        return opt
    }
    
    func unwrap(self): T {
        return self.value
    }
}

// Usage:
let maybe: Option<int32> = Option<int32>::some(42)
let val: int32 = maybe.unwrap()
```

## Generic Traits

```gecko
trait Container<T> {
    func get(self, index: uint64): T
    func set(self, index: uint64, value: T): void
    func length(self): uint64
}
```

## Trait Constraints

Constrain type parameters to types implementing a trait:

```gecko
func print_area<T is Shape>(shape: T*): void {
    printf("Area: %d\n", shape.area())
}
```

The compiler verifies at instantiation that the concrete type implements the trait.

### Multiple Trait Constraints

Use `&` to require multiple traits:

```gecko
func render<T is Drawable & Debug>(x: T): void {
    x.draw()
    x.debug_print()
}
```

## Monomorphization

Each unique combination of type arguments generates a specialized version:

```gecko
// These create two separate functions:
identity<int32>(42)
identity<bool>(true)

// Generated C code:
int32_t identity__int32_t(int32_t x) { return x; }
_Bool identity___Bool(_Bool x) { return x; }
```

## Multiple Type Parameters

```gecko
class Pair<A, B> {
    let first: A
    let second: B
}

func map<T, U>(value: T, f: func(T): U): U {
    return f(value)
}
```

## Nested Generics

```gecko
let nested: Option<Vec<int32>> = Option<Vec<int32>>::some(Vec<int32>::new())
```

## Gaps and Limitations

- Inference is bidirectional (arguments + expected type context), but not every advanced context is covered yet
- No variance annotations (covariance/contravariance)
- No `where` clauses
- No const generics (`Array<T, N>` where N is a compile-time integer)
- No higher-kinded types
- Code bloat from monomorphization (each instantiation duplicates code)
- No specialization (cannot provide optimized impl for specific types)
- Generic types must be fully specified at use site (no partial application)

## Inference Ambiguity

When generic arguments are not uniquely inferable, compilation fails with an
inference ambiguity diagnostic and a hint to provide explicit type arguments.

```gecko
func make_zero<T>(): T {
    let x: T
    return x
}

// Error: Type Inference Ambiguity (T cannot be inferred uniquely)
let v = make_zero()

// OK:
let v2: int32 = make_zero()
let v3 = make_zero<int32>()
```
