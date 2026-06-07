define ccc i64 @add(i64 %a, i64 %b) {
add$main:
	%0 = add i64 %a, %b
	ret i64 %0
}

define ccc void @test_wrong_type() {
test_wrong_type$main:
	%0 = call i64 @add([6 x i8] c"hello\00", i64 5)
	%result = alloca i64
	store i64 %0, i64* %result
	ret void
}

define ccc void @test_reassign_const() {
test_reassign_const$main:
	%x = alloca i64
	store i64 10, i64* %x
	ret void
}

define ccc void @test_wrong_assign() {
test_wrong_assign$main:
	%x = alloca i64
	store i64 10, i64* %x
	ret void
}
