@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define ccc i32 @color_code(i32 %c) {
color_code$main:
	ret i32 %c
}

define ccc i32 @main() {
main$main:
	%blue = alloca i32
	store i32 2, i32* %blue
	%red_code = alloca i32
	store i32 0, i32* %red_code
	%0 = load i32, i32* %blue
	%1 = call i32 @color_code(i32 %0)
	%2 = load i32, i32* %red_code
	%3 = add i32 %1, %2
	ret i32 %3
}
