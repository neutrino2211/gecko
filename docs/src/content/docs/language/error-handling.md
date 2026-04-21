---
title: Error Handling
description: Result types, Option types, and error propagation in Gecko
sidebar:
  order: 8
---

Gecko uses explicit error handling through `Result<T, E>` and `Option<T>` types, with hook-based syntax sugar for ergonomic usage.

## Result Type

`Result<T, E>` represents an operation that can succeed with value `T` or fail with error `E`.

```gecko
class Result<T, E> {
    let value: T
    let error: E
    let ok: bool
    
    public func ok(v: T): Result<T, E> {
        return Result { value: v, ok: true }
    }
    
    public func err(e: E): Result<T, E> {
        return Result { error: e, ok: false }
    }
    
    public func is_ok(self): bool { return self.ok }
    public func is_err(self): bool { return !self.ok }
    public func unwrap(self): T { return self.value }
    public func unwrap_err(self): E { return self.error }
}
```

### Basic Usage

```gecko
class FileError {
    let path: string
    let message: string
    
    public func not_found(path: string): FileError {
        return FileError { path: path, message: "file not found" }
    }
    
    public func permission_denied(path: string): FileError {
        return FileError { path: path, message: "permission denied" }
    }
}

func read_file(path: string): Result<string, FileError> {
    if !exists(path) {
        return Result::err(FileError::not_found(path))
    }
    if !readable(path) {
        return Result::err(FileError::permission_denied(path))
    }
    return Result::ok(read_contents(path))
}

// Explicit handling
let result = read_file("config.txt")
if result.is_err() {
    let e = result.unwrap_err()
    printf("Error: %s - %s\n", e.message, e.path)
    return
}
let content = result.unwrap()
```

## Option Type

`Option<T>` represents a value that may or may not exist.

```gecko
class Option<T> {
    let value: T
    let some: bool
    
    public func some(v: T): Option<T> {
        return Option { value: v, some: true }
    }
    
    public func none(): Option<T> {
        return Option { some: false }
    }
    
    public func is_some(self): bool { return self.some }
    public func is_none(self): bool { return !self.some }
    public func unwrap(self): T { return self.value }
    public func unwrap_or(self, default: T): T {
        if self.some { return self.value }
        return default
    }
}
```

### Basic Usage

```gecko
func find_user(id: int): Option<User> {
    let user = db_lookup(id)
    if user == null {
        return Option::none()
    }
    return Option::some(user)
}

// Explicit handling
let user_opt = find_user(42)
if user_opt.is_none() {
    printf("User not found\n")
    return
}
let user = user_opt.unwrap()
```

## The `try` Keyword

The `try` keyword unwraps a `Result` or `Option`, returning early if it contains an error or is empty.

### Try Hook

The `@try_hook` attribute enables the `try` keyword for a type. It requires two methods:
- `has_value(self): bool` - Returns true if the value is present
- `try_unwrap(self): T` - Returns the contained value

```gecko
@try_hook(.has_value, .try_unwrap)
trait Tryable<T> {
    func has_value(self): bool
    func try_unwrap(self): T
}

impl<T, E> Tryable<T> for Result<T, E> {
    func has_value(self): bool {
        return self.is_ok
    }
    func try_unwrap(self): T {
        return self.value
    }
}

impl<T> Tryable<T> for Option<T> {
    func has_value(self): bool {
        return self.some
    }
    func try_unwrap(self): T {
        return self.value
    }
}
```

### Usage with Result

```gecko
func process_file(path: string): Result<Data, FileError> {
    let content = try read_file(path)      // returns early if err
    let parsed = try parse_data(content)   // returns early if err
    let validated = try validate(parsed)   // returns early if err
    return Result::ok(validated)
}
```

The compiler rewrites `try expr` to (using GCC statement expressions):

```c
({
    ResultType __tmp = expr;
    if (!has_value(&__tmp)) {
        return __tmp;  // Early return the error/none
    }
    try_unwrap(&__tmp);  // Yields the unwrapped value
})
```

### Usage with Option

```gecko
func get_user_email(id: int): Option<string> {
    let user = try find_user(id)           // returns None if None
    let profile = try user.get_profile()   // returns None if None
    return Option::some(profile.email)
}
```

The same transformation applies to `Option`:

```c
({
    OptionType __tmp = expr;
    if (!has_value(&__tmp)) {
        return __tmp;  // Early return None
    }
    try_unwrap(&__tmp);  // Yields the unwrapped value
})
```

## The `or` Keyword

The `or` keyword provides a default value when a `Result` is an error or an `Option` is empty.

### Or Hook

The `@or_hook` attribute enables the `or` keyword for a type. It requires one method:
- `unwrap_or(self, default: T): T` - Returns the contained value or the default

```gecko
@or_hook(.unwrap_or)
trait Orable<T> {
    func unwrap_or(self, default: T): T
}

impl<T, E> Orable<T> for Result<T, E> {
    func unwrap_or(self, default: T): T {
        if self.is_ok { return self.value }
        return default
    }
}

impl<T> Orable<T> for Option<T> {
    func unwrap_or(self, default: T): T {
        if self.some { return self.value }
        return default
    }
}
```

### Usage

```gecko
// With Option
let name = get_username() or "anonymous"
let count = parse_int(input) or 0
let config = load_config() or default_config()

// With Result
let content = read_file("config.txt") or "{}"
let port = parse_port(env_var) or 8080
```

## Combining `try` and `or`

```gecko
func load_user_settings(id: int): Settings {
    // Try to load, fall back to defaults
    let user = find_user(id) or return default_settings()
    let prefs = user.get_preferences() or return default_settings()
    return prefs.to_settings()
}

// Or use try for propagation with or for specific defaults
func process(path: string): Result<Config, Error> {
    let content = try read_file(path)
    let port = parse_port(content) or 8080  // default if parse fails
    let host = parse_host(content) or "localhost"
    return Result::ok(Config { port: port, host: host })
}
```

## Error Types

Define custom error types as classes:

```gecko
class NetworkError {
    let code: int
    let message: string
    let url: string
    
    public func timeout(url: string): NetworkError {
        return NetworkError { 
            code: 408, 
            message: "request timed out", 
            url: url 
        }
    }
    
    public func not_found(url: string): NetworkError {
        return NetworkError { 
            code: 404, 
            message: "resource not found", 
            url: url 
        }
    }
    
    public func connection_failed(url: string): NetworkError {
        return NetworkError { 
            code: 0, 
            message: "connection failed", 
            url: url 
        }
    }
}

func fetch(url: string): Result<Response, NetworkError> {
    if !can_connect(url) {
        return Result::err(NetworkError::connection_failed(url))
    }
    // ...
}
```

## Summary

| Syntax | Meaning |
|--------|---------|
| `Result::ok(v)` | Create success result |
| `Result::err(e)` | Create error result |
| `Option::some(v)` | Create present option |
| `Option::none()` | Create empty option |
| `try expr` | Unwrap or early-return |
| `expr or default` | Unwrap or use default |
| `.unwrap()` | Unwrap (panic if empty/error) |
| `.is_ok()` / `.is_some()` | Check for success/presence |
| `.is_err()` / `.is_none()` | Check for error/absence |

## Best Practices

1. **Use `try` for propagation** - When errors should bubble up to the caller
2. **Use `or` for defaults** - When you have a sensible fallback value
3. **Use explicit checks** - When you need different handling for different error cases
4. **Create descriptive error types** - Include context (paths, codes, messages)
5. **Prefer `Result` over panics** - Reserve panics for unrecoverable situations
