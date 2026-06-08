define ccc i64 @main() {
main$main:
	%x = alloca i64
	store i64 42, i64* %x
	store i64 10, i64* %x
	ret i64 0
}
