%MultibootHeader = type { i32, i32, i32 }

@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define ccc i32 @main() {
main$main:
	%0 = alloca %MultibootHeader
	%1 = getelementptr %MultibootHeader, %MultibootHeader* %0, i32 0, i32 0
	store i32 464367618, i32* %1
	%2 = getelementptr %MultibootHeader, %MultibootHeader* %0, i32 0, i32 1
	store i32 0, i32* %2
	%3 = getelementptr %MultibootHeader, %MultibootHeader* %0, i32 0, i32 2
	store i32 3830599678, i32* %3
	%4 = load %MultibootHeader, %MultibootHeader* %0
	%header = alloca %MultibootHeader
	store %MultibootHeader %4, %MultibootHeader* %header
	ret i32 0
}
