@PI = constant i64 3

define ccc i64 @add(i64 %a, i64 %b) {
add$main:
	%0 = add i64 %a, %b
	ret i64 %0
}

define ccc i64 @mul(i64 %a, i64 %b) {
mul$main:
	%0 = mul i64 %a, %b
	ret i64 %0
}

define ccc i64 @test() {
test$main:
	%x = alloca i64
	%0 = load i64, i64* %x
	%1 = call i64 @add(i64 5, i64 %0)
	%result = alloca i64
	store i64 %1, i64* %result
	%2 = load i64, i64* %result
	%3 = call i64 @mul(i64 %2, i64 2)
	ret i64 %3
}
