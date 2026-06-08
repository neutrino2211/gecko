@.str.67221646275034566038851038082511 = private global [6 x i8] c"hello\00"
@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define ccc i32 @get_number() {
get_number$main:
	%0 = getelementptr [6 x i8], [6 x i8]* @.str.67221646275034566038851038082511, i8 0
	%1 = ptrtoint [6 x i8]* %0 to i32
	ret i32 %1
}

define ccc i32 @main() {
main$main:
	ret i32 0
}
