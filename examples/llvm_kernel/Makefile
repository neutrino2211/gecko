# Gecko LLVM Kernel Example
#
# Build:  make
# Run:    make run
#
# Requires LLVM toolchain:
#   brew install llvm          (macOS)
#   apt install llvm lld       (Linux)
#
# Run requires QEMU:
#   brew install qemu           (macOS)
#   apt install qemu-system-x86 (Linux)

GECKO = go run ../..
UNAME := $(shell uname)

ifneq ($(UNAME), Darwin)
  LLVM_CONFIG ?= llvm-config
else
  LLVM_CONFIG ?= /opt/homebrew/opt/llvm/bin/llvm-config
endif

LLC  ?= $(shell $(LLVM_CONFIG) --bindir)/llc
LLD  ?= $(shell $(LLVM_CONFIG) --bindir)/ld.lld

GECKO_FLAGS = --backend llvm --ir-only --target-arch i386 --target-platform none --target-vendor unknown

LLC_FLAGS = -relocation-model=static -code-model=small \
            -mattr=-sse,-sse2,-sse3,-ssse3,-sse4.1,-sse4.2,-avx,-avx2 \
            -O0

LDFLAGS = -m elf_i386 -T linker.ld -nostdlib

# Gecko source modules
GECKO_SOURCES = boot.gecko main.gecko io.gecko keyboard.gecko console.gecko shell.gecko cpu.gecko

# Output files
BOOT_LL = boot.ll
KERNEL_LL = kernel.ll
BOOT_O = boot.o
KERNEL_O = kernel.o
KERNEL_ELF = kernel.elf

OBJECTS = $(BOOT_O) $(KERNEL_O)

.PHONY: all clean run ll ir

all: $(KERNEL_ELF)

ir: $(BOOT_LL) $(KERNEL_LL)

ll: $(BOOT_LL) $(KERNEL_LL)

$(BOOT_LL): boot.gecko
	$(GECKO) compile $(GECKO_FLAGS) boot.gecko
	@if [ -f boot.gecko.ll ]; then mv boot.gecko.ll $(BOOT_LL); \
	elif [ -f .gecko_build/boot.gecko.ll ]; then cp .gecko_build/boot.gecko.ll $(BOOT_LL); \
	else echo "Error: boot.gecko.ll not found" >&2; exit 1; fi
	@echo "Generated $(BOOT_LL)"

$(KERNEL_LL): main.gecko io.gecko keyboard.gecko console.gecko shell.gecko cpu.gecko
	$(GECKO) compile $(GECKO_FLAGS) main.gecko
	@if [ -f main.gecko.ll ]; then mv main.gecko.ll $(KERNEL_LL); \
	elif [ -f .gecko_build/main.gecko.ll ]; then cp .gecko_build/main.gecko.ll $(KERNEL_LL); \
	else echo "Error: main.gecko.ll not found" >&2; exit 1; fi
	@echo "Generated $(KERNEL_LL)"

$(BOOT_O): $(BOOT_LL)
	$(LLC) $(LLC_FLAGS) -filetype=obj $(BOOT_LL) -o $@

$(KERNEL_O): $(KERNEL_LL)
	$(LLC) $(LLC_FLAGS) -filetype=obj $(KERNEL_LL) -o $@

$(KERNEL_ELF): $(OBJECTS) linker.ld
	$(LLD) $(LDFLAGS) -o $@ $(OBJECTS)
	@echo "Built $(KERNEL_ELF)"
	@echo "Kernel size: $$(wc -c < $@) bytes"

# Run with QEMU
run: $(KERNEL_ELF)
	qemu-system-x86_64 -kernel $(KERNEL_ELF) -m 32M
run-i386: $(KERNEL_ELF)
	qemu-system-i386 -kernel $(KERNEL_ELF) -m 32M

# Run with debug output
debug: $(KERNEL_ELF)
	qemu-system-x86_64 -kernel $(KERNEL_ELF) -m 32M -d int,cpu_reset -no-reboot

# Show generated LLVM IR
show-ir: ir
	@echo "=== boot.ll ===" && cat $(BOOT_LL)
	@echo "" && echo "=== kernel.ll ===" && cat $(KERNEL_LL)

# Section analysis
sections: $(KERNEL_ELF)
	llvm-objdump -h $(KERNEL_ELF)

# Disassembly
disasm: $(KERNEL_ELF)
	llvm-objdump -d $(KERNEL_ELF) | head -200

clean:
	rm -f $(BOOT_LL) $(KERNEL_LL) $(BOOT_O) $(KERNEL_O) $(KERNEL_ELF)
