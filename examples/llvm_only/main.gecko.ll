%Vec2 = type { i32, i32 }

@.str.77235808760528741150273578512366 = private global [4 x i8] c"%d\0A\00"

declare ccc i32 @printf(i8* %format, ...)

define i32 @sum(%Vec2* %self) {
sum$main:
	%0 = getelementptr %Vec2, %Vec2* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = getelementptr %Vec2, %Vec2* %self, i32 0, i32 1
	%3 = load i32, i32* %2
	%4 = add i32 %1, %3
	ret i32 %4
}

define ccc i32 @apply(%Vec2* %v, i32 %mode) {
apply$main:
	%0 = icmp eq i32 %mode, 0
	br i1 %0, label %if.then.1, label %if.merge.1

if.then.1:
	%1 = call i32 @sum(%Vec2* %v)
	ret i32 %1

if.merge.1:
	%2 = call i32 @sum(%Vec2* %v)
	%3 = add i32 %2, 10
	ret i32 %3
}

define ccc i32 @main() {
main$main:
	%v = alloca %Vec2
	%0 = getelementptr %Vec2, %Vec2* %v, i32 0, i32 0
	store i32 7, i32* %0
	%1 = getelementptr %Vec2, %Vec2* %v, i32 0, i32 1
	store i32 5, i32* %1
	%2 = call i32 @apply(%Vec2* %v, i32 0)
	%out_add = alloca i32
	store i32 %2, i32* %out_add
	%3 = call i32 @apply(%Vec2* %v, i32 1)
	%out_bias = alloca i32
	store i32 %3, i32* %out_bias
	%4 = getelementptr [4 x i8], [4 x i8]* @.str.77235808760528741150273578512366, i8 0
	%5 = load i32, i32* %out_add
	%6 = load i32, i32* %out_bias
	%7 = add i32 %5, %6
	%8 = call i32 (i8*, ...) @printf([4 x i8]* %4, i32 %7)
	ret i32 0
}
