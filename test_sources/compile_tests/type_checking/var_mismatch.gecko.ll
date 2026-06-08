@.str.82003540671136784076475320541156 = private global [6 x i8] c"hello\00"

define ccc i64 @main() {
main$main:
	%x = alloca i64
	store i64 42, i64* %x
	%0 = getelementptr [6 x i8], [6 x i8]* @.str.82003540671136784076475320541156, i8 0
	%1 = ptrtoint [6 x i8]* %0 to i64
	store i64 %1, i64* %x
	ret i64 0
}
