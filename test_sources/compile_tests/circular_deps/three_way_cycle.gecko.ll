%B = type opaque
%A = type { %B }
%C = type opaque
%B = type { %C }
%C = type { %A }

@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define ccc i32 @main() {
main$main:
	ret i32 0
}
