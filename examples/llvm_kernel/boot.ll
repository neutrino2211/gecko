target triple = "i386-unknown-none-elf"

@stack_bottom = global [16384 x i8] zeroinitializer
@llvm.used = appending global [2 x i8*] [i8* bitcast (void ()* @kmain to i8*), i8* bitcast (void ()* @_start to i8*)], section "llvm.metadata"

define void @__multiboot_header() naked section ".multiboot" {
__multiboot_header$main:
	call void asm sideeffect ".align 4", ""()
	call void asm sideeffect ".long 0x1BADB002", ""()
	call void asm sideeffect ".long 0x00000000", ""()
	call void asm sideeffect ".long 0xE4524FFE", ""()
	ret void
}

declare void @kmain()

define void @_start() naked noreturn section ".text.boot" {
_start$main:
	call void asm sideeffect "lea stack_bottom + 16384, %esp", ""()
	call void asm sideeffect "pushl $$0", ""()
	call void asm sideeffect "popf", ""()
	call void asm sideeffect "call kmain", ""()
	call void asm sideeffect "cli", ""()
	call void asm sideeffect ".hang:", ""()
	call void asm sideeffect "hlt", ""()
	call void asm sideeffect "jmp .hang", ""()
	ret void
}
