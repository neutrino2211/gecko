---
title: Control Flow
description: Conditionals and loops in Gecko
sidebar:
  order: 2
---

## Conditionals

### If Statements

Parentheses around conditions are optional:

```ts
if x > 0 {
    printf("positive\n")
}

// Parentheses work too (treated as expression)
if (x > 0) {
    printf("positive\n")
}
```

### If-Else

```ts
if x > 0 {
    printf("positive\n")
} else {
    printf("non-positive\n")
}
```

### Else-If Chains

```ts
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

```ts
let i: int = 0
for i < 10 {
    printf("%d\n", i)
    i = i + 1
}
```

### While Loops

```ts
let i: int = 0
while i < 10 {
    printf("%d\n", i)
    i = i + 1
}
```

### Break and Continue

```ts
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

```ts
if x > 0 && x < 100 {
    printf("x is between 0 and 100\n")
}

if x < 0 || x > 100 {
    printf("x is out of range\n")
}
```

### Short-Circuit Evaluation

Logical operators short-circuit:

```ts
// safe_divide is only called if x != 0
if x != 0 && safe_divide(100, x) > 10 {
    printf("result is large\n")
}
```
