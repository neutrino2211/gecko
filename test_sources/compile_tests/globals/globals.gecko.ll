@MULTIBOOT_MAGIC = constant i32 464367618, section ".multiboot"
@MULTIBOOT_FLAGS = constant i32 3, section ".multiboot"
@MULTIBOOT_CHECKSUM = constant i32 3830599675, section ".multiboot"
@global_counter = global i32 0
@CONST_VALUE = constant i32 3735928559, section ".rodata"
@MULTIBOOT_HEADER_END = external global i32 0, section ".multiboot"
@kernel_stack = global [16384 x i8] zeroinitializer, section ".bss"
@stack_top = global i64 0
@MAX_BUFFER_SIZE = constant i32 u0x1000

define ccc i32 @test_globals() {
test_globals$main:
	%0 = load i32, i32* @MULTIBOOT_MAGIC
	%local_var = alloca i32
	store i32 %0, i32* %local_var
	%1 = load i32, i32* %local_var
	ret i32 %1
}

define ccc i64 @main() {
main$main:
	%0 = call i32 @test_globals()
	%result = alloca i32
	store i32 %0, i32* %result
	ret i64 0
}
