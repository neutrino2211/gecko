# Hello Kernel - Gecko OS Example

A minimal x86 kernel written in Gecko that displays "Hello, Gecko!" on screen.

## Features Demonstrated

- **Packed structs** - Multiboot header with exact memory layout
- **Global constants** - VGA colors and addresses
- **Section attributes** - `.bss` for stack, `.text.boot` for entry point
- **Fixed-size arrays** - 16KB kernel stack
- **Volatile pointers** - VGA memory-mapped I/O
- **Pointer casts** - Integer to pointer for MMIO addresses
- **Array indexing** - Writing to VGA buffer
- **Inline assembly** - CPU control (cli, hlt)
- **Function attributes** - `@naked`, `@noreturn`, `@section`

## Prerequisites

- **Gecko compiler** (this repository)
- **GCC** with 32-bit support (`gcc-multilib` on Ubuntu/Debian)
- **GNU LD** (binutils)
- **QEMU** for testing (`qemu-system-i386`)
- **GRUB** for ISO creation (optional, `grub-pc-bin` + `xorriso`)

### Ubuntu/Debian Setup

```bash
sudo apt install gcc-multilib binutils qemu-system-x86 grub-pc-bin xorriso
```

### macOS Setup

```bash
# Install cross-compiler (x86_64-elf-gcc recommended for kernel dev)
brew install x86_64-elf-gcc qemu

# Update Makefile to use x86_64-elf-gcc instead of gcc
```

## Building

```bash
# Generate C code and view it
make show-c

# Build the kernel ELF
make

# Build bootable ISO (requires GRUB)
make iso
```

## Running

```bash
# Run directly in QEMU (uses -kernel flag)
make run

# Run from ISO (more realistic boot)
make run-iso
```

You should see "Hello, Gecko!" displayed in green on a black screen.

## Project Structure

```
hello_kernel/
├── kernel.gecko    # Kernel source code
├── linker.ld       # Linker script (memory layout)
├── Makefile        # Build system
└── README.md       # This file
```

## How It Works

1. **Boot**: GRUB/QEMU loads the kernel at 1MB (`0x100000`)
2. **Entry**: `_start` is called (naked function, no prologue)
3. **Setup**: `kmain` disables interrupts
4. **Display**: Writes "Hello, Gecko!" to VGA text buffer at `0xB8000`
5. **Halt**: CPU halts with interrupts disabled

## Generated C Code

The Gecko compiler generates clean, readable C:

```c
typedef struct __attribute__((packed)) {
    uint32_t magic;
    uint32_t flags;
    uint32_t checksum;
} MultibootHeader;

__attribute__((section(".bss"))) uint8_t kernel_stack[16384];

void kernel__vga_putchar_at(uint8_t c, uint8_t color, int64_t x, int64_t y) {
    volatile uint16_t* vga = ((volatile uint16_t*)(0xB8000));
    int64_t index = y * 80 + x;
    vga[index] = ((uint16_t)(c)) | (((uint16_t)(color)) << 8);
}

__attribute__((naked)) __attribute__((noreturn)) 
__attribute__((section(".text.boot"))) 
void kernel___start() {
    __asm__ volatile ("call kmain");
    __asm__ volatile ("cli");
    __asm__ volatile ("hlt");
}
```

## Next Steps

- Add keyboard input handling
- Implement a simple shell
- Add memory management (paging)
- Implement interrupts (IDT)

## Troubleshooting

**"gcc: error: unrecognized command line option '-m32'"**
- Install 32-bit support: `sudo apt install gcc-multilib`

**QEMU shows nothing**
- Check that the kernel compiles without errors
- Try `make disasm` to verify code generation

**Triple fault / reboot loop**
- Check linker script alignment
- Verify multiboot header is in first 8KB
