define ccc i32* @bad_ptr() {
bad_ptr$main:
	%x = alloca i32
	store i32 42, i32* %x
	ret i32* %x
}

define ccc i64 @main() {
main$main:
	ret i64 0
}
