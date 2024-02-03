@.str.02526884054625787655230000854871 = private global [12 x i8] c"A CONST %s\0A\00"
@.str.64781035476232385486082657360664 = private global [10 x i8] c"A let %d\0A\00"

declare external ccc void @printf(i8* %format, i64 %a1)

define ccc i64 @main() {
main$main:
	%0 = getelementptr [12 x i8], [12 x i8]* @.str.02526884054625787655230000854871, i8 0
	call void @printf([12 x i8]* %0, [2 x i8] c"H\00")
	%1 = getelementptr [10 x i8], [10 x i8]* @.str.64781035476232385486082657360664, i8 0
	call void @printf([10 x i8]* %1, i64 40)
	ret i64 0
}
