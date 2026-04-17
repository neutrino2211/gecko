# Traits Example

This example demonstrates Gecko's trait system with geometric shapes.

## Features Shown

1. **Trait Definitions** - Define behavior contracts
2. **Trait Implementations** - Implement traits for classes
3. **Method Dispatch** - Call trait methods on instances
4. **Trait Constraints** - Use traits as generic bounds

## Building and Running

```bash
# Build
gecko build shapes.gecko -o shapes

# Run
./shapes
echo $?  # Expected: 93 (50 + 16 + 27)

# Or run directly
gecko run shapes.gecko
```

## Code Overview

### Traits
- `Area` - requires an `area(self): int` method
- `Perimeter` - requires a `perimeter(self): int` method

### Classes
- `Rectangle` - width and height
- `Square` - side length  
- `Circle` - radius (uses pi ≈ 3 for integer math)

### Generic Functions
- `double_area<T is Area>` - doubles the area of any shape implementing Area
- `sum_dimensions<T is Area>` - sums areas of two shapes of the same type

## Expected Output

Exit code 93:
- Rectangle area: 10 * 5 = 50
- Square area: 4 * 4 = 16
- Circle area: 3 * 3 * 3 = 27
- Total: 50 + 16 + 27 = 93
