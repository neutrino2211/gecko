define ccc i64 @test_break() {
test_break$main:
	%i = alloca i64
	store i64 0, i64* %i
	%result = alloca i64
	store i64 0, i64* %result
	br label %loop.header.1

loop.header.1:
	%0 = load i64, i64* %i
	%1 = icmp slt i64 %0, 100
	br i1 %1, label %loop.body.1, label %loop.exit.1

loop.body.1:
	%2 = load i64, i64* %i
	%3 = icmp eq i64 %2, 10
	br i1 %3, label %if.then.2, label %if.merge.2

loop.exit.1:
	%4 = load i64, i64* %result
	ret i64 %4

if.then.2:
	br label %loop.exit.1

if.merge.2:
	%5 = load i64, i64* %result
	%6 = load i64, i64* %i
	%7 = add i64 %5, %6
	store i64 %7, i64* %result
	%8 = load i64, i64* %i
	%9 = add i64 %8, 1
	store i64 %9, i64* %i
	br label %loop.header.1

loop.break.dead.3:
	br label %if.merge.2
}

define ccc i64 @test_continue() {
test_continue$main:
	%i = alloca i64
	store i64 0, i64* %i
	%result = alloca i64
	store i64 0, i64* %result
	br label %loop.header.4

loop.header.4:
	%0 = load i64, i64* %i
	%1 = icmp slt i64 %0, 10
	br i1 %1, label %loop.body.4, label %loop.exit.4

loop.body.4:
	%2 = load i64, i64* %i
	%3 = add i64 %2, 1
	store i64 %3, i64* %i
	%4 = load i64, i64* %i
	%5 = icmp eq i64 %4, 5
	br i1 %5, label %if.then.5, label %if.merge.5

loop.exit.4:
	%6 = load i64, i64* %result
	ret i64 %6

if.then.5:
	br label %loop.header.4

if.merge.5:
	%7 = load i64, i64* %result
	%8 = load i64, i64* %i
	%9 = add i64 %7, %8
	store i64 %9, i64* %result
	br label %loop.header.4

loop.continue.dead.6:
	br label %if.merge.5
}
