@.str.62155066001881480236677007312335 = private global [19 x i8] c"Another number %d\0A\00"
@.str.07706532071200013720435667563218 = private global [13 x i8] c"A number %d\0A\00"
@.str.57386676364818780036674824687336 = private global [13 x i8] c"A string %s\0A\00"
@.str.05610270252163716520553770318821 = private global [3 x i8] c"HH\00"

declare ccc void @printf(i8* %format, ...)

define ccc i64 @main() {
main$main:
	%0 = getelementptr [19 x i8], [19 x i8]* @.str.62155066001881480236677007312335, i8 0
	%1 = getelementptr [13 x i8], [13 x i8]* @.str.07706532071200013720435667563218, i8 0
	call void @printf([13 x i8]* %1, i64 90)
	%2 = getelementptr [13 x i8], [13 x i8]* @.str.57386676364818780036674824687336, i8 0
	%3 = getelementptr [3 x i8], [3 x i8]* @.str.05610270252163716520553770318821, i8 0
	call void @printf([13 x i8]* %2, [3 x i8]* %3)
	call void @printf([19 x i8]* %0, i64 30)
	ret i64 30
}
