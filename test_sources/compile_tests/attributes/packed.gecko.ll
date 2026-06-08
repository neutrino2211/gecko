%MultibootHeader = type <{ i32, i32, i32 }>
%NormalStruct = type { i8, i32, i8 }

@header = global i32 0, section ".multiboot"

define ccc i64 @main() {
main$main:
	%x = alloca i32
	store i32 42, i32* %x
	ret i64 0
}
