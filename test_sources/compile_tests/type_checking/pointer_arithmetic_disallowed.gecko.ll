@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define ccc i32 @main() {
main$main:
	%0 = inttoptr i64 0 to i8*
	%p = alloca i8*
	store i8* %0, i8** %p
	%1 = load i8*, i8** %p
	%2 = add i8* %1, 1
	%q = alloca i8*
	store i8* %2, i8** %q
	ret i32 0
}
