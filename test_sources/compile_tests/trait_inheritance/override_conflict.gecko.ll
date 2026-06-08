%Item = type { i32 }

@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define i1 @Item__Child__value(%Item* %self) {
Item__Child__value$main:
	ret i1 true
}

define ccc i32 @main() {
main$main:
	ret i32 0
}
