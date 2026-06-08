%Data = type { i64 }

define ccc i64 @main() {
main$main:
	%a = alloca %Data
	%0 = getelementptr %Data, %Data* %a, i32 0, i32 0
	store i64 42, i64* %0
	%1 = load %Data, %Data* %a
	%b = alloca %Data
	store %Data %1, %Data* %b
	%2 = getelementptr %Data, %Data* %a, i32 0, i32 0
	%3 = load i64, i64* %2
	%x = alloca i64
	store i64 %3, i64* %x
	%4 = load i64, i64* %x
	%5 = getelementptr %Data, %Data* %b, i32 0, i32 0
	%6 = load i64, i64* %5
	%7 = add i64 %4, %6
	ret i64 %7
}
