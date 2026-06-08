@.str.64860112425434388580863551444472 = private global [2 x i8] c"0\00"
@.str.34283274832866543486611172526147 = private global [3 x i8] c"42\00"
@.str.73234466556503005606163727071506 = private global [3 x i8] c"30\00"
@.str.12375857220262231456242651021188 = private global [2 x i8] c"1\00"
@.str.53202668375511355883008203870618 = private global [13 x i8] c"flag is true\00"
@.str.38476418640276657504345631578004 = private global [18 x i8] c"Hello, inference!\00"
@.str.68384150366756517210237371538517 = private global [11 x i8] c"x equals y\00"
@.str.76356066444087782017852283885773 = private global [18 x i8] c"negative is false\00"
@llvm.used = appending global [2 x i8*] [i8* bitcast (i32 (i8*)* @puts to i8*), i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

declare ccc i32 @puts(i8* %s)

define ccc void @print_int(i32 %val) {
print_int$main:
	%0 = trunc i64 0 to i32
	%1 = icmp eq i32 %val, %0
	br i1 %1, label %if.then.1, label %if.merge.1

if.then.1:
	%2 = getelementptr [2 x i8], [2 x i8]* @.str.64860112425434388580863551444472, i8 0
	%3 = bitcast [2 x i8]* %2 to i8*
	%4 = call i32 @puts(i8* %3)
	ret void

if.merge.1:
	%5 = trunc i64 42 to i32
	%6 = icmp eq i32 %val, %5
	br i1 %6, label %if.then.2, label %if.merge.2

if.then.2:
	%7 = getelementptr [3 x i8], [3 x i8]* @.str.34283274832866543486611172526147, i8 0
	%8 = bitcast [3 x i8]* %7 to i8*
	%9 = call i32 @puts(i8* %8)
	br label %if.merge.2

if.merge.2:
	%10 = trunc i64 30 to i32
	%11 = icmp eq i32 %val, %10
	br i1 %11, label %if.then.3, label %if.merge.3

if.then.3:
	%12 = getelementptr [3 x i8], [3 x i8]* @.str.73234466556503005606163727071506, i8 0
	%13 = bitcast [3 x i8]* %12 to i8*
	%14 = call i32 @puts(i8* %13)
	br label %if.merge.3

if.merge.3:
	%15 = trunc i64 1 to i32
	%16 = icmp eq i32 %val, %15
	br i1 %16, label %if.then.4, label %if.merge.4

if.then.4:
	%17 = getelementptr [2 x i8], [2 x i8]* @.str.12375857220262231456242651021188, i8 0
	%18 = bitcast [2 x i8]* %17 to i8*
	%19 = call i32 @puts(i8* %18)
	br label %if.merge.4

if.merge.4:
	ret void
}

define ccc i32 @main() {
main$main:
	%x = alloca i32
	store i32 42, i32* %x
	%0 = load i32, i32* %x
	call void @print_int(i32 %0)
	%flag = alloca i1
	store i1 true, i1* %flag
	%1 = load i1, i1* %flag
	br i1 %1, label %if.then.5, label %if.merge.5

if.then.5:
	%2 = getelementptr [13 x i8], [13 x i8]* @.str.53202668375511355883008203870618, i8 0
	%3 = bitcast [13 x i8]* %2 to i8*
	%4 = call i32 @puts(i8* %3)
	br label %if.merge.5

if.merge.5:
	%5 = getelementptr [18 x i8], [18 x i8]* @.str.38476418640276657504345631578004, i8 0
	%msg = alloca i8*
	%6 = bitcast [18 x i8]* %5 to i8*
	store i8* %6, i8** %msg
	%7 = load i8*, i8** %msg
	%8 = call i32 @puts(i8* %7)
	%9 = add i32 10, 20
	%sum = alloca i32
	store i32 %9, i32* %sum
	%10 = load i32, i32* %sum
	call void @print_int(i32 %10)
	%11 = load i32, i32* %x
	%y = alloca i32
	store i32 %11, i32* %y
	%12 = load i32, i32* %y
	call void @print_int(i32 %12)
	%13 = load i32, i32* %y
	%14 = load i32, i32* %x
	%15 = icmp eq i32 %14, %13
	%isEqual = alloca i1
	store i1 %15, i1* %isEqual
	%16 = load i1, i1* %isEqual
	br i1 %16, label %if.then.6, label %if.merge.6

if.then.6:
	%17 = getelementptr [11 x i8], [11 x i8]* @.str.68384150366756517210237371538517, i8 0
	%18 = bitcast [11 x i8]* %17 to i8*
	%19 = call i32 @puts(i8* %18)
	br label %if.merge.6

if.merge.6:
	%20 = load i1, i1* %flag
	%21 = xor i1 %20, true
	%negative = alloca i1
	store i1 %21, i1* %negative
	%22 = load i1, i1* %negative
	%23 = xor i1 %22, true
	br i1 %23, label %if.then.7, label %if.merge.7

if.then.7:
	%24 = getelementptr [18 x i8], [18 x i8]* @.str.76356066444087782017852283885773, i8 0
	%25 = bitcast [18 x i8]* %24 to i8*
	%26 = call i32 @puts(i8* %25)
	br label %if.merge.7

if.merge.7:
	ret i32 0
}
