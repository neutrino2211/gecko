%Inner = type { i32 }
%Outer = type { %Inner* }

define i32 @double(%Inner* %self) {
double$main:
	%0 = getelementptr %Inner, %Inner* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = getelementptr %Inner, %Inner* %self, i32 0, i32 0
	%3 = load i32, i32* %2
	%4 = add i32 %1, %3
	ret i32 %4
}

define i32 @get_double(%Outer* %self) {
get_double$main:
	%0 = getelementptr %Outer, %Outer* %self, i32 0, i32 0
	%1 = load %Inner*, %Inner** %0
	%2 = call i32 @double(%Inner* %1)
	ret i32 %2
}

define ccc i32 @main() {
main$main:
	%inner = alloca %Inner
	%0 = getelementptr %Inner, %Inner* %inner, i32 0, i32 0
	store i32 21, i32* %0
	%outer = alloca %Outer
	%1 = getelementptr %Outer, %Outer* %outer, i32 0, i32 0
	store %Inner* %inner, %Inner** %1
	%2 = getelementptr %Outer, %Outer* %outer, i32 0, i32 0
	%3 = load %Inner*, %Inner** %2
	%4 = call i32 @double(%Inner* %3)
	%5 = getelementptr %Outer, %Outer* %outer, i32 0, i32 0
	%6 = load %Inner*, %Inner** %5
	%7 = call i32 @double(%Inner* %6)
	%from_expr = alloca i32
	store i32 %7, i32* %from_expr
	%8 = load i32, i32* %from_expr
	ret i32 %8
}
