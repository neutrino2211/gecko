{ i32, i32 } = type { i32, i32 }

define i32 @sum({ i32, i32 }* %self) {
sum$main:
	%0 = getelementptr { i32, i32 }, { i32, i32 }* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = getelementptr { i32, i32 }, { i32, i32 }* %self, i32 0, i32 1
	%3 = load i32, i32* %2
	%4 = add i32 %1, %3
	ret i32 %4
}

define ccc i32 @apply({ i32, i32 }* %v, i32 %mode) {
apply$main:
	%0 = icmp eq i32 %mode, 0
	br i1 %0, label %if.then.1, label %if.merge.1

if.then.1:
	%1 = call i32 @sum({ i32, i32 }* %v)
	ret i32 %1

if.merge.1:
	%2 = call i32 @sum({ i32, i32 }* %v)
	%3 = add i32 %2, 10
	ret i32 %3
}

define ccc i32 @main() {
main$main:
	%v = alloca { i32, i32 }
	%0 = getelementptr { i32, i32 }, { i32, i32 }* %v, i32 0, i32 0
	store i32 7, i32* %0
	%1 = getelementptr { i32, i32 }, { i32, i32 }* %v, i32 0, i32 1
	store i32 5, i32* %1
	%2 = call i32 @apply({ i32, i32 }* %v, i32 0)
	%out_add = alloca i32
	store i32 %2, i32* %out_add
	%3 = call i32 @apply({ i32, i32 }* %v, i32 1)
	%out_bias = alloca i32
	store i32 %3, i32* %out_bias
	%4 = load i32, i32* %out_add
	%5 = load i32, i32* %out_bias
	%6 = add i32 %4, %5
	ret i32 %6
}
