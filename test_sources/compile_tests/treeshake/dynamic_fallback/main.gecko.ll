%T = type opaque

@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define ccc i64 @dyn_add(i64 %a, i64 %b) {
dyn_add$main:
	%0 = add i64 %a, %b
	ret i64 %0
}

define ccc i8* @dyn_apply(i8* (i8*, i8*)* %f, i8* %x, i8* %y) {
dyn_apply$main:
	%0 = call i8* %f(i8* %x, i8* %y)
	ret i8* %0
}

define ccc i64 @dyn_unreachable() {
dyn_unreachable$main:
	ret i64 99
}

define ccc i32 @main() {
main$main:
	%0 = bitcast i64 (i64, i64)* @dyn_add to i8* (i8*, i8*)*
	%1 = inttoptr i64 20 to i8*
	%2 = inttoptr i64 22 to i8*
	%3 = call i8* @dyn_apply(i8* (i8*, i8*)* %0, i8* %1, i8* %2)
	%4 = ptrtoint i8* %3 to i32
	ret i32 %4
}
