%Option = type { i8*, i1 }
%File = type {}

@.str.73045381114604126000600342658466 = private global [9 x i8] c"no_exist\00"
@.str.25382423652002535641584587884847 = private global [2 x i8] c"r\00"
@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define %Option @main__Option__some(i8* %val) {
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

define ccc %Option @main__File__open(i8* %path, i8* %mode) {
main__File__open$main:
	%0 = call %Option @main__Option__none()
	ret %Option %0
}

define ccc i32 @read_file() {
read_file$main:
	%0 = getelementptr [9 x i8], [9 x i8]* @.str.73045381114604126000600342658466, i8 0
	%1 = bitcast [9 x i8]* %0 to i8*
	%2 = getelementptr [2 x i8], [2 x i8]* @.str.25382423652002535641584587884847, i8 0
	%3 = bitcast [2 x i8]* %2 to i8*
	%4 = call %Option @main__File__open(i8* %1, i8* %3)
	%5 = alloca %Option
	store %Option %4, %Option* %5
	%6 = getelementptr %Option, %Option* %5, i32 0, i32 0
	%7 = load i8*, i8** %6
	%8 = ptrtoint i8* %7 to i32
	%fd = alloca i32
	store i32 %8, i32* %fd
	%9 = load i32, i32* %fd
	ret i32 %9
}

define ccc i32 @main() {
main$main:
	ret i32 0
}
