%MyClass = type { i32 }

@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define i32 @MyClass__TraitA__do_thing(%MyClass* %self) {
MyClass__TraitA__do_thing$main:
	%0 = getelementptr %MyClass, %MyClass* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	ret i32 %1
}

define i32 @MyClass__TraitB__do_thing(%MyClass* %self) {
MyClass__TraitB__do_thing$main:
	%0 = getelementptr %MyClass, %MyClass* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = mul i32 %1, 2
	ret i32 %2
}

define ccc i32 @main() {
main$main:
	ret i32 0
}
