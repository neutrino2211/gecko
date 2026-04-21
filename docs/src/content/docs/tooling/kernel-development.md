---
title: Kernel Development
description: Writing operating system kernels with Gecko
sidebar:
  order: 4
---

Gecko provides low-level features for writing operating system kernels and bare-metal code.

## Getting Started

See the `examples/hello_kernel/` directory for a complete x86 kernel example.

### Project Setup

```toml
# gecko.toml
[package]
name = "mykernel"
version = "0.1.0"

[build]
backend = "c"

[build.entries]
kernel = "src/kernel.gecko"

[target.i386-none-elf]
freestanding = true
linker_script = "linker.ld"
```

## Low-Level Attributes

### @packed

Remove padding from struct layouts for hardware compatibility:

```gecko
@packed
class GDTEntry {
    let limit_low: uint16
    let base_low: uint16
    let base_middle: uint8
    let access: uint8
    let granularity: uint8
    let base_high: uint8
}
```

### @section

Place code or data in specific memory sections:

```gecko
@section(".multiboot")
const MULTIBOOT_HEADER: [3]uint32 = [
    0x1BADB002,  // Magic
    0x00000003,  // Flags
    0xE4524FFB   // Checksum
]

@section(".text.boot")
@naked
func _start(): void {
    // Boot code
}
```

### @naked

Create functions without prologue/epilogue:

```gecko
@naked
func interrupt_handler(): void {
    asm {
        "pusha"
        "call handle_interrupt"
        "popa"
        "iret"
    }
}
```

### @noreturn

Mark functions that never return:

```gecko
@noreturn
func panic(msg: string): void {
    // Print error message
    // Halt the system
    asm { "cli" "hlt" }
}
```

## Volatile Memory Access

For memory-mapped I/O (MMIO):

```gecko
// VGA text buffer
const VGA_BUFFER: uint16 volatile* = 0xB8000 as uint16 volatile*

func write_char(pos: int, c: uint8, color: uint8): void {
    let entry: uint16 = (color as uint16 << 8) | (c as uint16)
    @write_volatile(VGA_BUFFER + pos, entry)
}

// Read hardware status
func read_status(): uint8 {
    let status_reg: uint8 volatile* = 0x3F8 + 5 as uint8 volatile*
    return @read_volatile(status_reg)
}
```

## Inline Assembly

Embed assembly directly in Gecko code:

```gecko
func enable_interrupts(): void {
    asm { "sti" }
}

func disable_interrupts(): void {
    asm { "cli" }
}

func halt(): void {
    asm { "hlt" }
}

func outb(port: uint16, value: uint8): void {
    asm {
        "mov dx, %0"
        "mov al, %1"
        "out dx, al"
        : // no outputs
        : "r"(port), "r"(value)
    }
}
```

## Fixed-Size Arrays

For static data structures:

```gecko
// Page directory
let page_directory: [1024]uint32

// Interrupt descriptor table
let idt: [256]IDTEntry

// Static buffer
@section(".bss")
let kernel_stack: [4096]uint8
```

## VGA Console Example

```gecko
package kernel

const VGA_WIDTH: int = 80
const VGA_HEIGHT: int = 25
const VGA_ADDRESS: uint64 = 0xB8000

const BLACK: uint8 = 0
const LIGHT_GRAY: uint8 = 7

@packed
class Console {
    let buffer: uint16 volatile*
    let cursor_x: int
    let cursor_y: int
    let color: uint8
}

func make_color(fg: uint8, bg: uint8): uint8 {
    return fg | (bg << 4)
}

func make_entry(c: uint8, color: uint8): uint16 {
    return (c as uint16) | ((color as uint16) << 8)
}

impl Console {
    func new(): Console {
        return Console {
            buffer: VGA_ADDRESS as uint16 volatile*,
            cursor_x: 0,
            cursor_y: 0,
            color: make_color(LIGHT_GRAY, BLACK)
        }
    }

    func clear(self): void {
        let blank = make_entry(0x20, self.color)
        let i: int = 0
        while i < VGA_WIDTH * VGA_HEIGHT {
            @write_volatile(self.buffer + i, blank)
            i = i + 1
        }
        self.cursor_x = 0
        self.cursor_y = 0
    }

    func putchar(self, c: uint8): void {
        if c == 0x0A {  // Newline
            self.cursor_x = 0
            self.cursor_y = self.cursor_y + 1
        } else {
            let index = self.cursor_y * VGA_WIDTH + self.cursor_x
            @write_volatile(self.buffer + index, make_entry(c, self.color))
            self.cursor_x = self.cursor_x + 1
            if self.cursor_x >= VGA_WIDTH {
                self.cursor_x = 0
                self.cursor_y = self.cursor_y + 1
            }
        }
    }

    func print(self, s: string): void {
        let i: int = 0
        while s[i] != 0 {
            self.putchar(s[i])
            i = i + 1
        }
    }
}
```

## Linker Script

```ld
/* linker.ld */
ENTRY(_start)

SECTIONS {
    . = 0x100000;  /* Load at 1MB */

    .multiboot : {
        *(.multiboot)
    }

    .text : {
        *(.text.boot)
        *(.text)
    }

    .rodata : {
        *(.rodata)
    }

    .data : {
        *(.data)
    }

    .bss : {
        *(COMMON)
        *(.bss)
    }
}
```

## Building the Kernel

```bash
# Compile
gecko build --target-arch=i386 --target-platform=none \
    --target-vendor=none-elf --entry kernel -o kernel.elf

# Create bootable image (with GRUB)
mkdir -p iso/boot/grub
cp kernel.elf iso/boot/
cat > iso/boot/grub/grub.cfg << EOF
menuentry "My Kernel" {
    multiboot /boot/kernel.elf
}
EOF
grub-mkrescue -o mykernel.iso iso/

# Run in QEMU
qemu-system-i386 -cdrom mykernel.iso
```

## Tips

1. **Test in QEMU first** - Much faster iteration than real hardware
2. **Use serial output for debugging** - More reliable than VGA
3. **Keep early boot code minimal** - Complex code before paging is risky
4. **Document hardware assumptions** - Memory layout, interrupt numbering, etc.
5. **Use @packed sparingly** - Only for hardware structures
