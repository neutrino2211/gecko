{ i32 } = type { i32 }

define i32 @Meter__Metric__score({ i32 }* %self) {
Meter__Metric__score$main:
	%0 = getelementptr { i32 }, { i32 }* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = mul i32 %1, 2
	ret i32 %2
}

define ccc { i32 } @make_meter(i32 %v) {
make_meter$main:
	%0 = alloca { i32 }
	%1 = getelementptr { i32 }, { i32 }* %0, i32 0, i32 0
	store i32 %v, i32* %1
	ret { i32 }* %0
}

define ccc i32 @main() {
main$main:
	%0 = call { i32 } @make_meter(i64 21)
	%m = alloca { i32 }
	store { i32 } %0, { i32 }* %m
	%1 = call i32 @Meter__Metric__score({ i32 }* %m)
	%2 = call i32 @Meter__Metric__score({ i32 }* %m)
	ret i32 %2
}
