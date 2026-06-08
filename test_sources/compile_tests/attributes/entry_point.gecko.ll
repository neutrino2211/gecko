define ccc void @_start() naked noreturn {
_start$main:
	call void asm sideeffect "hlt", ""()
	ret void
}

define ccc void @boot() section ".text.boot" {
boot$main:
	call void asm sideeffect "nop", ""()
	ret void
}
