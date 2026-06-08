define ccc void @halt() {
halt$main:
	call void asm sideeffect "hlt", ""()
	ret void
}

define ccc i64 @main() {
main$main:
	call void @halt()
	ret i64 0
}
