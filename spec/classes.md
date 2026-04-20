# Classes

Classes in Gecko are value types (like C structs) with associated methods.

## Declaration Syntax

```gecko
[visibility] [external "name"] class Name[<TypeParams>] {
    fields...
    methods...
}
```

## Basic Class

```gecko
class Point {
    let x: int32
    let y: int32
    
    func new(x: int32, y: int32): Point {
        let p: Point
        p.x = x
        p.y = y
        return p
    }
    
    func distance(self, other: Point*): int32 {
        let dx: int32 = self.x - other.x
        let dy: int32 = self.y - other.y
        return dx * dx + dy * dy
    }
}
```

## Fields

### Field Declarations

```gecko
class Buffer {
    let data: uint8*
    let len: uint64
    let cap: uint64
}
```

### Field Visibility

```gecko
class User {
    public let name: string
    private let password_hash: uint64
}
```

**Gap**: Visibility is parsed but not enforced.

## Methods

### Static Methods

Methods without `self` parameter are static:

```gecko
class Vec {
    func new(): Vec {
        let v: Vec
        v.data = nil
        v.len = 0
        return v
    }
}

// Called as:
let v: Vec = Vec::new()
```

### Instance Methods

Methods with `self` parameter operate on instances:

```gecko
class Vec {
    func push(self, value: int32): void {
        // self is Vec*
        self.data[self.len] = value
        self.len = self.len + 1
    }
    
    func length(self): uint64 {
        return self.len
    }
}

// Called as:
v.push(42)
let n: uint64 = v.length()
```

**Note**: `self` is always passed as a pointer.

## Construction

No constructors. Use static factory methods:

```gecko
class Rect {
    let width: int32
    let height: int32
    
    func new(w: int32, h: int32): Rect {
        let r: Rect
        r.width = w
        r.height = h
        return r
    }
}

let rect: Rect = Rect::new(10, 20)
```

### Struct Literals

Direct initialization:

```gecko
let rect: Rect = Rect { width: 10, height: 20 }
```

## External Classes

Map to C structs:

```gecko
external "struct termios" class Termios {
    let c_iflag: uint32
    let c_oflag: uint32
    let c_cflag: uint32
    let c_lflag: uint32
}
```

## Generic Classes

See [generics.md](generics.md).

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
}
```

## Attributes

### @packed

Remove padding between fields:

```gecko
@packed
class Header {
    let magic: uint16
    let version: uint8
    let flags: uint8
}
```

## Gaps and Limitations

- No inheritance
- No destructors (use `Drop` trait manually)
- No member initialization in declaration
- No private/protected enforcement
- No `const` methods
- Classes are always value types (no reference semantics)
