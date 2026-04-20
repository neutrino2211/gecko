# Control Flow

## Conditionals

### If Statement

```gecko
if condition {
    // body
}
```

### If-Else

```gecko
if condition {
    // then branch
} else {
    // else branch
}
```

### If-Else If-Else

```gecko
if condition1 {
    // ...
} else if condition2 {
    // ...
} else if condition3 {
    // ...
} else {
    // ...
}
```

### Condition Expression

Any expression that evaluates to `bool`:

```gecko
if x > 0 {
    // ...
}

if ptr != nil {
    // ...
}

if is_valid(data) {
    // ...
}
```

**Note**: No implicit truthiness. Must be explicitly boolean.

## Loops

### While Loop

```gecko
while condition {
    // body
}
```

Example:

```gecko
let i: int32 = 0
while i < 10 {
    printf("%d\n", i)
    i = i + 1
}
```

### For Loop

C-style for loop:

```gecko
for init; condition; post {
    // body
}
```

Example:

```gecko
for let i: int32 = 0; i < 10; i = i + 1 {
    printf("%d\n", i)
}
```

**Gap**: No iterator-based for loop (`for item in collection`).

### Break

Exit innermost loop:

```gecko
while true {
    if done {
        break
    }
}
```

### Continue

Skip to next iteration:

```gecko
let i: int32 = 0
while i < 10 {
    i = i + 1
    if i == 5 {
        continue
    }
    printf("%d\n", i)
}
```

## Return

### Return Value

```gecko
func add(a: int32, b: int32): int32 {
    return a + b
}
```

### Void Return

```gecko
func log(msg: string): void {
    puts(msg)
    return
}
```

Implicit return at end of void functions.

### Early Return

```gecko
func find(arr: int32*, len: int32, target: int32): int32 {
    let i: int32 = 0
    while i < len {
        if arr[i] == target {
            return i  // early exit
        }
        i = i + 1
    }
    return -1
}
```

## Gaps and Limitations

- No `switch`/`match` statement
- No labeled breaks (`break 'outer`)
- No `loop` keyword (infinite loop)
- No iterator-based for loop
- No `do-while` loop
- No conditional expressions (`x = cond ? a : b`)
- No `if` as expression
- No pattern matching
- No `defer` statement
