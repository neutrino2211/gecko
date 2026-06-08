%Circle = type { i64 }

@.str.07503582007731322510382183406143 = private global [6 x i8] c"hello\00"

define ccc i64 @main() {
main$main:
	%0 = alloca %Circle
	%1 = getelementptr %Circle, %Circle* %0, i32 0, i32 0
	store i64 5, i64* %1
	%2 = load %Circle, %Circle* %0
	%c = alloca %Circle
	store %Circle %2, %Circle* %c
	%3 = getelementptr %Circle, %Circle* %c, i32 0, i32 0
	%4 = getelementptr [6 x i8], [6 x i8]* @.str.07503582007731322510382183406143, i8 0
	%5 = ptrtoint [6 x i8]* %4 to i64
	store i64 %5, i64* %3
	ret i64 0
}
