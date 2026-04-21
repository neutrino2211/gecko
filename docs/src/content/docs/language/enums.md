---
title: Enums
description: Enumeration types in Gecko
sidebar:
  order: 8
---

Enums define a type with a fixed set of named values.

## Defining Enums

```gecko
enum Color {
    Red
    Green
    Blue
}

enum Status {
    Pending
    Active
    Completed
    Cancelled
}
```

## Using Enums

Access enum variants with the `::` operator:

```gecko
let primary: Color = Color::Red
let state: Status = Status::Active
```

## Enums as Types

Use enums as function parameters and return types:

```gecko
func status_message(s: Status): string {
    // Implementation depends on how you handle the value
    return "Status received"
}

func get_default_color(): Color {
    return Color::Blue
}
```

## Enum Values

Enum variants are represented as integers internally, starting from 0:

```gecko
enum Priority {
    Low      // 0
    Medium   // 1
    High     // 2
    Critical // 3
}
```

## Common Patterns

### State Machines

```gecko
enum ConnectionState {
    Disconnected
    Connecting
    Connected
    Error
}

class Connection {
    let state: ConnectionState
    let address: string
}

impl Connection {
    func new(addr: string): Connection {
        return Connection {
            state: ConnectionState::Disconnected,
            address: addr
        }
    }

    func connect(self): void {
        self.state = ConnectionState::Connecting
        // ... perform connection
        self.state = ConnectionState::Connected
    }

    func disconnect(self): void {
        self.state = ConnectionState::Disconnected
    }

    func is_connected(self): bool {
        // Compare enum values
        return self.state == ConnectionState::Connected
    }
}
```

### Options and Results

The standard library provides `Option<T>` for optional values:

```gecko
import std.option use { Option }

func find_user(id: int): Option<User> {
    if id == 0 {
        return Option::none()
    }
    return Option::some(User { id: id })
}

// Usage
let user = find_user(42)
if user.is_some() {
    let u = user.unwrap()
    // use u
}
```

### Command Patterns

```gecko
enum Command {
    Start
    Stop
    Pause
    Resume
}

func execute(cmd: Command): void {
    // Handle each command
}

execute(Command::Start)
execute(Command::Stop)
```

## Comparing Enums

Enum values can be compared for equality:

```gecko
let c1: Color = Color::Red
let c2: Color = Color::Red
let c3: Color = Color::Blue

if c1 == c2 {
    // true - both are Red
}

if c1 != c3 {
    // true - Red != Blue
}
```

## Best Practices

1. **Use enums for fixed sets of values** - When you have a known, limited set of options
2. **Name variants descriptively** - `ConnectionState::Connected` is clearer than `State::S2`
3. **Consider Option<T>** - For values that might be absent, use the stdlib Option type
4. **Group related enums** - Keep related enums in the same module
