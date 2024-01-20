@.str.20543423032554076220847883035776 = global [19 x i8] c"Another number %d\0A\00"
@.str.21732248330326740844084634502177 = global [13 x i8] c"A number %d\0A\00"
@.str.18465145027871744835634124707357 = global [13 x i8] c"A string %s\0A\00"
@.str.40666262813845852380031160444450 = global [3 x i8] c"HH\00"

declare external ccc void @printf(i8* %format, ...)

define ccc i64 @main() {
main$main:
	%0 = getelementptr [19 x i8], [19 x i8]* @.str.20543423032554076220847883035776, i8 0
	%1 = getelementptr [13 x i8], [13 x i8]* @.str.21732248330326740844084634502177, i8 0
	call void (i8*, ...) @printf([13 x i8]* %1, i64 90)
	%2 = getelementptr [13 x i8], [13 x i8]* @.str.18465145027871744835634124707357, i8 0
	%3 = getelementptr [3 x i8], [3 x i8]* @.str.40666262813845852380031160444450, i8 0
	call void (i8*, ...) @printf([13 x i8]* %2, [3 x i8]* %3)
	call void (i8*, ...) @printf([19 x i8]* %0, i64 30)
	ret i64 30
}
