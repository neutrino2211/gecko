{ i32 } = type { i32 }
{ { i32 }* } = type { { i32 }* }

define i32 @double({ i32 }* %self) {
double$main:
	%0 = getelementptr { i32 }, { i32 }* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = getelementptr { i32 }, { i32 }* %self, i32 0, i32 0
	%3 = load i32, i32* %2
	%4 = add i32 %1, %3
	ret i32 %4
}

define i32 @get_double({ { i32 }* }* %self) {
get_double$main:
	%0 = getelementptr { { i32 }* }, { { i32 }* }* %self, i32 0, i32 0
	%1 = load { i32 }*, { i32 }** %0
	%2 = call i32 @double({ i32 }* %1)
	ret i32 %2
}

define ccc i32 @main() {
main$main:
	%inner = alloca { i32 }
	%0 = getelementptr { i32 }, { i32 }* %inner, i32 0, i32 0
	store i32 21, i32* %0
	%outer = alloca { { i32 }* }
	%1 = getelementptr { { i32 }* }, { { i32 }* }* %outer, i32 0, i32 0
	store { i32 }* %inner, { i32 }** %1
	%2 = getelementptr { { i32 }* }, { { i32 }* }* %outer, i32 0, i32 0
	%3 = load { i32 }*, { i32 }** %2
	%4 = call i32 @double({ i32 }* %3)
	%5 = getelementptr { { i32 }* }, { { i32 }* }* %outer, i32 0, i32 0
	%6 = load { i32 }*, { i32 }** %5
	%7 = call i32 @double({ i32 }* %6)
	%from_expr = alloca i32
	store i32 %7, i32* %from_expr
	%8 = load i32, i32* %from_expr
	ret i32 %8
}
