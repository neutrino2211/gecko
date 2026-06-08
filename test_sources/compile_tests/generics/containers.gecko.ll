%Box = type { i8*, i1 }
%Pair = type { i8*, i8* }
%Triple = type { i8*, i8*, i8* }
%T = type opaque
%A = type opaque
%B = type opaque

@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define ccc i8* @identity(i8* %x) {
identity$main:
	ret i8* %x
}

define ccc i8* @first(i8* %a, i8* %b) {
first$main:
	ret i8* %a
}

define ccc i8* @second(i8* %a, i8* %b) {
second$main:
	ret i8* %b
}

define ccc i64 @add_ints(i64 %a, i64 %b) {
add_ints$main:
	%0 = add i64 %a, %b
	ret i64 %0
}

define ccc i8* @apply(i8* (i8*, i8*)* %f, i8* %x, i8* %y) {
apply$main:
	%0 = call i8* %f(i8* %x, i8* %y)
	ret i8* %0
}

define ccc i32 @main() {
main$main:
	%box_int = alloca %Box
	%box_byte = alloca %Box
	%pair = alloca %Pair
	%triple = alloca %Triple
	%nested = alloca %Box
	%0 = inttoptr i64 10 to i8*
	%1 = call i8* @identity(i8* %0)
	%x = alloca i64
	%2 = ptrtoint i8* %1 to i64
	store i64 %2, i64* %x
	%3 = inttoptr i64 20 to i8*
	%4 = call i8* @identity(i8* %3)
	%y = alloca i8
	%5 = ptrtoint i8* %4 to i8
	store i8 %5, i8* %y
	%6 = inttoptr i64 1 to i8*
	%7 = inttoptr i64 2 to i8*
	%8 = call i8* @first(i8* %6, i8* %7)
	%a = alloca i64
	%9 = ptrtoint i8* %8 to i64
	store i64 %9, i64* %a
	%10 = inttoptr i64 1 to i8*
	%11 = inttoptr i64 2 to i8*
	%12 = call i8* @second(i8* %10, i8* %11)
	%b = alloca i8
	%13 = ptrtoint i8* %12 to i8
	store i8 %13, i8* %b
	%14 = bitcast i64 (i64, i64)* @add_ints to i8* (i8*, i8*)*
	%15 = inttoptr i64 5 to i8*
	%16 = inttoptr i64 3 to i8*
	%17 = call i8* @apply(i8* (i8*, i8*)* %14, i8* %15, i8* %16)
	%sum = alloca i64
	%18 = ptrtoint i8* %17 to i64
	store i64 %18, i64* %sum
	%19 = load i64, i64* %x
	%20 = load i64, i64* %a
	%21 = load i64, i64* %sum
	%22 = add i64 %20, %21
	%23 = add i64 %19, %22
	%24 = trunc i64 %23 to i32
	ret i32 %24
}
