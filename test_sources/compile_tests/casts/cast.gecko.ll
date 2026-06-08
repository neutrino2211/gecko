@.str.20228730253301481126165366386048 = private global [12 x i8] c"Result: %d\0A\00"
@llvm.used = appending global [1 x i8*] [i8* bitcast (i64 (i8*, ...)* @printf to i8*)], section "llvm.metadata"

declare ccc i64 @printf(i8* %format, ...)

define ccc i64 @test_cast(i64 %x) {
test_cast$main:
	%small = alloca i64
	store i64 %x, i64* %small
	%0 = load i64, i64* %small
	ret i64 %0
}

define ccc i64 @main() {
main$main:
	%0 = call i64 @test_cast(i64 42)
	%result = alloca i64
	store i64 %0, i64* %result
	%1 = getelementptr [12 x i8], [12 x i8]* @.str.20228730253301481126165366386048, i8 0
	%2 = bitcast [12 x i8]* %1 to i8*
	%3 = load i64, i64* %result
	%4 = call i64 (i8*, ...) @printf(i8* %2, i64 %3)
	ret i64 0
}
