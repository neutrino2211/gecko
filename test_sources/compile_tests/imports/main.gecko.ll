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
	%0 = load i64, i64* @PI
	%x = alloca i64
	store i64 %0, i64* %x
	%1 = load i64, i64* %x
	%2 = call i64 @add(i64 5, i64 %1)
	%result = alloca i64
	store i64 %2, i64* %result
	%3 = load i64, i64* %result
	%4 = call i64 @mul(i64 %3, i64 2)
	ret i64 %4
}
