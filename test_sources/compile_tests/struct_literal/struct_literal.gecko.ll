%MultibootHeader = type { i32, i32, i32 }

define ccc i32 @main() {
main$main:
	%header = alloca %MultibootHeader
	%0 = getelementptr %MultibootHeader, %MultibootHeader* %header, i32 0, i32 0
	store i32 464367618, i32* %0
	%1 = getelementptr %MultibootHeader, %MultibootHeader* %header, i32 0, i32 1
	store i32 0, i32* %1
	%2 = getelementptr %MultibootHeader, %MultibootHeader* %header, i32 0, i32 2
	store i32 3830599678, i32* %2
	ret i32 0
}
