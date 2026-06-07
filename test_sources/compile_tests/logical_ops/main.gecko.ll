@.str.52246212764113045802577653871870 = private global [24 x i8] c"Logical operators work!\00"

declare ccc i32 @puts(i8* %s)

define ccc i32 @main() {
main$main:
	%a = alloca i1
	store i1 true, i1* %a
	%b = alloca i1
	store i1 false, i1* %b
	%0 = load i1, i1* %a
	%1 = load i1, i1* %b
	%2 = and i1 %0, %1
	br i1 %2, label %if.then.1, label %if.merge.1

if.then.1:
	ret i32 1

if.merge.1:
	%3 = load i1, i1* %a
	%4 = load i1, i1* %b
	%5 = or i1 %3, %4
	%6 = xor i1 %5, true
	br i1 %6, label %if.then.2, label %if.merge.2

if.then.2:
	ret i32 2

if.merge.2:
	%c = alloca i1
	store i1 true, i1* %c
	%7 = load i1, i1* %a
	%8 = load i1, i1* %c
	%9 = and i1 %7, %8
	%10 = xor i1 %9, true
	br i1 %10, label %if.then.3, label %if.merge.3

if.then.3:
	ret i32 3

if.merge.3:
	%11 = load i1, i1* %b
	%12 = load i1, i1* %c
	%13 = or i1 %11, %12
	br i1 %13, label %if.then.4, label %if.else.4

if.then.4:
	br label %if.merge.4

if.merge.4:
	%14 = getelementptr [24 x i8], [24 x i8]* @.str.52246212764113045802577653871870, i8 0
	%15 = call i32 @puts([24 x i8]* %14)
	ret i32 0

if.else.4:
	ret i32 4
}
