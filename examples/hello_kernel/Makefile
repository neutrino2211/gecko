# Gecko Kernel Example Makefile
#
# IMPORTANT: Kernel development requires an ELF cross-compiler.
# macOS's clang doesn't support ELF sections needed for kernels.
#
# On macOS, install: brew install x86_64-elf-gcc x86_64-elf-binutils
# Then use: make CC=x86_64-elf-gcc LD=x86_64-elf-ld AS=x86_64-elf-as
#
# On Linux (Ubuntu/Debian): sudo apt install gcc-multilib nasm

# Tools - auto-detect cross-compiler on macOS
GECKO = go run ../..
UNAME := $(shell uname)

ifeq ($(UNAME), Darwin)
  # macOS: use cross-compiler (install with: brew install x86_64-elf-gcc x86_64-elf-binutils)
  CC ?= x86_64-elf-gcc
  AS ?= x86_64-elf-as
  LD ?= x86_64-elf-ld
  OBJCOPY ?= x86_64-elf-objcopy
else
  # Linux: use native gcc with -m32
  CC ?= gcc
  AS ?= as
  LD ?= ld
  OBJCOPY ?= objcopy
endif

# Flags
GECKO_FLAGS = --backend c

ifeq ($(UNAME), Darwin)
  # Cross-compiler flags (no -m32 needed, target is already i386/x86_64)
  CFLAGS = -m32 -ffreestanding -fno-stack-protector -fno-pie -nostdlib \
           -Wall -Wextra -O2
  ASFLAGS = --32
  LDFLAGS = -m elf_i386 -T linker.ld -nostdlib
else
  # Native Linux gcc flags
  CFLAGS = -m32 -ffreestanding -fno-stack-protector -fno-pie -nostdlib \
           -Wall -Wextra -O2
  ASFLAGS = --32
  LDFLAGS = -m elf_i386 -T linker.ld -nostdlib
endif

# Files
KERNEL_SRC = main.gecko
KERNEL_C = kernel.c
KERNEL_O = kernel.o
BOOT_S = boot.S
BOOT_O = boot.o
KERNEL_ELF = kernel.elf
KERNEL_BIN = kernel.bin
ISO = gecko_os.iso

OBJECTS = $(BOOT_O) $(KERNEL_O)

.PHONY: all clean run iso c

all: $(KERNEL_ELF)

# Step 1: Gecko -> C
c:
	$(GECKO) compile $(GECKO_FLAGS) --ir-only $(KERNEL_SRC)
	@mv main.gecko.c $(KERNEL_C) 2>/dev/null || true
	@echo "Generated $(KERNEL_C)"

# Assemble boot.S
$(BOOT_O): $(BOOT_S)
	$(AS) $(ASFLAGS) $< -o $@

# Compile kernel C code
$(KERNEL_O): $(KERNEL_C)
	$(CC) $(CFLAGS) -c $(KERNEL_C) -o $@

# Ensure C is generated
$(KERNEL_C):
	$(MAKE) c

# Link everything
$(KERNEL_ELF): $(OBJECTS) linker.ld
	$(LD) $(LDFLAGS) -o $@ $(OBJECTS)

# Create flat binary
$(KERNEL_BIN): $(KERNEL_ELF)
	$(OBJCOPY) -O binary $< $@

# Create bootable ISO (requires grub-mkrescue)
iso: $(KERNEL_ELF)
	mkdir -p isodir/boot/grub
	cp $(KERNEL_ELF) isodir/boot/kernel.elf
	echo 'menuentry "Gecko OS" { multiboot /boot/kernel.elf }' > isodir/boot/grub/grub.cfg
	grub-mkrescue -o $(ISO) isodir
	rm -rf isodir

# Run in QEMU
run: $(KERNEL_ELF)
	qemu-system-i386 -kernel $(KERNEL_ELF)

# Run ISO in QEMU
run-iso: $(ISO)
	qemu-system-i386 -cdrom $(ISO)

# Clean
clean:
	rm -f $(KERNEL_C) $(KERNEL_O) $(BOOT_O) $(KERNEL_ELF) $(KERNEL_BIN) $(ISO) kernel.gecko.c
	rm -rf isodir

# Show generated C
show-c: c
	@cat $(KERNEL_C)

# Debug helpers
sections: $(KERNEL_ELF)
	objdump -h $(KERNEL_ELF)

disasm: $(KERNEL_ELF)
	objdump -d $(KERNEL_ELF)
