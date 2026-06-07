{ i32 } = type { i32 }

define i32 @Probe__ChildMetric__score({ i32 }* %self) {
Probe__ChildMetric__score$main:
	%0 = getelementptr { i32 }, { i32 }* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = add i32 %1, 100
	ret i32 %2
}

define ccc { i32 } @make_probe(i32 %v) {
make_probe$main:
	%0 = alloca { i32 }
	%1 = getelementptr { i32 }, { i32 }* %0, i32 0, i32 0
	store i32 %v, i32* %1
	ret { i32 }* %0
}

define ccc i32 @main() {
main$main:
	%0 = call { i32 } @make_probe(i64 23)
	%p = alloca { i32 }
	store { i32 } %0, { i32 }* %p
	%1 = call i32 @Probe__ChildMetric__score({ i32 }* %p)
	%2 = call i32 @Probe__ChildMetric__score({ i32 }* %p)
	ret i32 %2
}
