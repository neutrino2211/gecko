---
title: Control Flow
description: Conditionals and loops in Gecko
sidebar:
  order: 2
---

## Conditionals

### If Statements

Parentheses around conditions are optional:

```gecko
if x > 0 {
    printf("positive\n")
}

// Parentheses work too (treated as expression)
if (x > 0) {
    printf("positive\n")
}
```

### If-Else

```gecko
if x > 0 {
    printf("positive\n")
} else {
    printf("non-positive\n")
}
```

### Else-If Chains

```gecko
if x > 0 {
    printf("positive\n")
} else if x < 0 {
    printf("negative\n")
} else {
    printf("zero\n")
}
```

## Loops

### For Loops

The `for` loop is the primary loop construct:

```gecko
let i: int = 0
for i < 10 {
    printf("%d\n", i)
    i = i + 1
}
```

### While Loops

```gecko
let i: int = 0
while i < 10 {
    printf("%d\n", i)
    i = i + 1
}
```

### For-In Loops

Iterate over any type that implements the `Iterator` trait:

```gecko
import std.collections.vec use { Vec }

let numbers: Vec<int> = Vec::new()
numbers.push(1)
numbers.push(2)
numbers.push(3)

for let n in numbers {
    printf("%d\n", n)
}
```

The `for-in` loop works with any type implementing `Iterator<T>`:

```gecko
import std.collections.string use { String }

let s = String::from("hello")
for let c in s {
    // c is each byte (uint8) in the string
}
```

You can also iterate over custom types by implementing the `Iterator` trait:

```gecko
import std.core.traits use { Iterator }

class Range {
    let current: int
    let end: int
}

impl Iterator<int> for Range {
    func next(self): int {
        let val = self.current
        self.current = self.current + 1
        return val
    }

    func has_next(self): bool {
        return self.current < self.end
    }
}

// Usage
let r = Range { current: 0, end: 5 }
for let i in r {
    printf("%d\n", i)  // Prints 0, 1, 2, 3, 4
}
```

### Break and Continue

```gecko
let i: int = 0
for i < 100 {
    if i == 10 {
        break      // Exit loop
    }
    if i == 5 {
        i = i + 1
        continue   // Skip to next iteration
    }
    printf("%d\n", i)
    i = i + 1
}
```

## Logical Operators

Use `&&` (and) and `||` (or) for compound conditions:

```gecko
if x > 0 && x < 100 {
    printf("x is between 0 and 100\n")
}

if x < 0 || x > 100 {
    printf("x is out of range\n")
}
```

### Short-Circuit Evaluation

Logical operators short-circuit:

```gecko
// safe_divide is only called if x != 0
if x != 0 && safe_divide(100, x) > 10 {
    printf("result is large\n")
}
```
