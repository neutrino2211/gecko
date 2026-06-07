{ i32 } = type { i32 }

define i32 @inc({ i32 }* %self) {
inc$main:
	%0 = getelementptr { i32 }, { i32 }* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = add i32 %1, 1
	ret i32 %2
}

define ccc { i32 } @make_counter(i32 %v) {
make_counter$main:
	%0 = alloca { i32 }
	%1 = getelementptr { i32 }, { i32 }* %0, i32 0, i32 0
	store i32 %v, i32* %1
	ret { i32 }* %0
}

define ccc i32 @main() {
main$main:
	%0 = call { i32 } @make_counter(i64 41)
	%c = alloca { i32 }
	store { i32 } %0, { i32 }* %c
	%1 = call i32 @inc({ i32 }* %c)
	%2 = call i32 @inc({ i32 }* %c)
	ret i32 %2
}
