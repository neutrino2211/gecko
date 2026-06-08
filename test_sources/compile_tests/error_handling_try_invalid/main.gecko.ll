%Option = type { i8*, i1 }
%T = type opaque

@llvm.used = appending global [2 x i8*] [i8* bitcast (i32 (i8*)* @puts to i8*), i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

declare ccc i32 @puts(i8* %s)

define ccc %Option @main__Option__some(i8* %val) {
main__Option__some$main:
	%opt = alloca %Option
	%0 = getelementptr %Option, %Option* %opt, i32 0, i32 0
	store i8* %val, i8** %0
	%1 = getelementptr %Option, %Option* %opt, i32 0, i32 1
	store i1 true, i1* %1
	%2 = load %Option, %Option* %opt
	ret %Option %2
}

define %Option @main__Option__none() {
main__Option__none$main:
	%opt = alloca %Option
	%0 = getelementptr %Option, %Option* %opt, i32 0, i32 1
	store i1 false, i1* %0
	%1 = load %Option, %Option* %opt
	ret %Option %1
}

define i1 @Option__Tryable__T__has_value(%Option* %self) {
Option__Tryable__T__has_value$main:
	%0 = getelementptr %Option, %Option* %self, i32 0, i32 1
	%1 = load i1, i1* %0
	ret i1 %1
}

define i8* @Option__Tryable__T__try_unwrap(%Option* %self) {
Option__Tryable__T__try_unwrap$main:
	%0 = getelementptr %Option, %Option* %self, i32 0, i32 0
	%1 = load i8*, i8** %0
	ret i8* %1
}

define ccc %Option @get_value() {
get_value$main:
	%0 = inttoptr i64 42 to i8*
	%1 = call %Option @main__Option__some(i8* %0)
	ret %Option %1
}

define ccc i32 @bad_function() {
bad_function$main:
	%0 = call %Option @get_value()
	%1 = alloca %Option
	store %Option %0, %Option* %1
	%2 = getelementptr %Option, %Option* %1, i32 0, i32 0
	%3 = load i8*, i8** %2
	%4 = ptrtoint i8* %3 to i32
	%val = alloca i32
	store i32 %4, i32* %val
	%5 = load i32, i32* %val
	ret i32 %5
}

define ccc i32 @main() {
main$main:
	ret i32 0
}
