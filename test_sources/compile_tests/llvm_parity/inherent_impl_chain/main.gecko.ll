%Counter = type { i32 }

@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define i32 @provider__Counter__inc(%Counter* %self) {
provider__Counter__inc$main:
	%0 = getelementptr %Counter, %Counter* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = add i32 %1, 1
	ret i32 %2
}

define ccc %Counter @make_counter(i32 %v) {
make_counter$main:
	%0 = alloca %Counter
	%1 = getelementptr %Counter, %Counter* %0, i32 0, i32 0
	store i32 %v, i32* %1
	%2 = load %Counter, %Counter* %0
	ret %Counter %2
}

define ccc i32 @main() {
main$main:
	%0 = call %Counter @make_counter(i32 41)
	%c = alloca %Counter
	store %Counter %0, %Counter* %c
	%1 = call i32 @provider__Counter__inc(%Counter* %c)
	%2 = call i32 @provider__Counter__inc(%Counter* %c)
	ret i32 %2
}
