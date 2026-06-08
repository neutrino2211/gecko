@.str.83263022680614502821477030467865 = private global [10 x i8] c"Mainasara\00"
@.str.11421360802023617203314412658761 = private global [36 x i8] c"Address of string in Gecko is 0x%x\0A\00"
@.str.20236737513811513526713322406438 = private global [24 x i8] c"The greeting is\0A=> \22%s\22\00"
@llvm.used = appending global [2 x i8*] [i8* bitcast (i8* (i8*)* @get_greeting to i8*), i8* bitcast (void (i8*, ...)* @printf to i8*)], section "llvm.metadata"

declare ccc i8* @get_greeting(i8* %name)

declare ccc void @printf(i8* %format, ...)

define ccc i64 @main() {
main$main:
	%0 = getelementptr [10 x i8], [10 x i8]* @.str.83263022680614502821477030467865, i8 0
	%1 = bitcast [10 x i8]* %0 to i8*
	%2 = call i8* @get_greeting(i8* %1)
	%greeting = alloca i8*
	store i8* %2, i8** %greeting
	%3 = getelementptr [36 x i8], [36 x i8]* @.str.11421360802023617203314412658761, i8 0
	%4 = bitcast [36 x i8]* %3 to i8*
	%5 = load i8*, i8** %greeting
	call void (i8*, ...) @printf(i8* %4, i8* %5)
	%6 = getelementptr [24 x i8], [24 x i8]* @.str.20236737513811513526713322406438, i8 0
	%7 = bitcast [24 x i8]* %6 to i8*
	%8 = load i8*, i8** %greeting
	call void (i8*, ...) @printf(i8* %7, i8* %8)
	ret i64 0
}
