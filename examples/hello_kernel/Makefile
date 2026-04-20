# Gecko Kernel Example - Modern Interactive Kernel
#
# Features:
#   - VGA text console with Index hooks
#   - PS/2 keyboard input
#   - Simple shell with commands
#
# Build: make
# Run:   make run
#
# Requires cross-compiler on macOS:
#   brew install x86_64-elf-gcc x86_64-elf-binutils

GECKO = go run ../..
UNAME := $(shell uname)

ifeq ($(UNAME), Darwin)
  CC ?= x86_64-elf-gcc
  LD ?= x86_64-elf-ld
  OBJCOPY ?= x86_64-elf-objcopy
else
  CC ?= gcc
  LD ?= ld
  OBJCOPY ?= objcopy
endif

GECKO_FLAGS = --backend c

CFLAGS = -m32 -ffreestanding -fno-stack-protector -fno-pie -nostdlib \
         -Wall -Wextra -O2 -fno-builtin -mno-sse -mno-sse2 -mno-mmx
LDFLAGS = -m elf_i386 -T linker.ld -nostdlib

# Gecko source modules
GECKO_SOURCES = boot.gecko main.gecko io.gecko keyboard.gecko console.gecko shell.gecko cpu.gecko

# Output files
BOOT_C = boot.c
KERNEL_C = kernel.c
BOOT_O = boot.o
KERNEL_O = kernel.o
KERNEL_ELF = kernel.elf
ISO = gecko_os.iso

OBJECTS = $(BOOT_O) $(KERNEL_O)

.PHONY: all clean run iso c debug

all: $(KERNEL_ELF)

c: $(BOOT_C) $(KERNEL_C)

$(BOOT_C): boot.gecko
	$(GECKO) compile $(GECKO_FLAGS) --ir-only boot.gecko
	@mv boot.gecko.c $(BOOT_C)
	@echo "Generated $(BOOT_C)"

$(KERNEL_C): main.gecko io.gecko keyboard.gecko console.gecko shell.gecko cpu.gecko
	$(GECKO) compile $(GECKO_FLAGS) --ir-only main.gecko
	@mv main.gecko.c $(KERNEL_C)
	@echo "Generated $(KERNEL_C)"

$(BOOT_O): $(BOOT_C)
	$(CC) $(CFLAGS) -c $(BOOT_C) -o $@

$(KERNEL_O): $(KERNEL_C)
	$(CC) $(CFLAGS) -c $(KERNEL_C) -o $@

$(KERNEL_ELF): $(OBJECTS) linker.ld
	$(LD) $(LDFLAGS) -o $@ $(OBJECTS)
	@echo "Built $(KERNEL_ELF)"

iso: $(KERNEL_ELF)
	mkdir -p isodir/boot/grub
	cp $(KERNEL_ELF) isodir/boot/kernel.elf
	echo 'set timeout=0' > isodir/boot/grub/grub.cfg
	echo 'menuentry "Gecko OS" { multiboot /boot/kernel.elf }' >> isodir/boot/grub/grub.cfg
	grub-mkrescue -o $(ISO) isodir 2>/dev/null
	rm -rf isodir
	@echo "Created $(ISO)"

# Run with QEMU (x86_64 in 32-bit compat mode)
run: $(KERNEL_ELF)
	qemu-system-x86_64 -kernel $(KERNEL_ELF) -m 32M

# Run with debug output
debug: $(KERNEL_ELF)
	qemu-system-x86_64 -kernel $(KERNEL_ELF) -m 32M -d int,cpu_reset -no-reboot

# Run ISO
run-iso: $(ISO)
	qemu-system-x86_64 -cdrom $(ISO) -m 32M

clean:
	rm -f $(BOOT_C) $(KERNEL_C) $(BOOT_O) $(KERNEL_O) $(KERNEL_ELF) $(ISO)
	rm -rf isodir

# Show generated C code
show-c: c
	@echo "=== boot.c ===" && cat $(BOOT_C)
	@echo "" && echo "=== kernel.c ===" && cat $(KERNEL_C)

# Debug helpers
sections: $(KERNEL_ELF)
	objdump -h $(KERNEL_ELF)

disasm: $(KERNEL_ELF)
	objdump -d $(KERNEL_ELF) | head -200
