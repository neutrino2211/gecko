@.str.30470822845700106762304227245531 = private global [24 x i8] c"Addition: %d + %d = %d\0A\00"
@.str.26764145785832063616415870381021 = private global [27 x i8] c"Subtraction: %d - %d = %d\0A\00"
@.str.63538466015350376607021102052383 = private global [30 x i8] c"Multiplication: %d * %d = %d\0A\00"
@.str.14141123841086581601010064820708 = private global [24 x i8] c"Division: %d / %d = %d\0A\00"
@.str.05026335814357373843080833558252 = private global [27 x i8] c"Bitwise AND: %d & %d = %d\0A\00"
@.str.68031407054182712361870168706145 = private global [26 x i8] c"Bitwise OR: %d | %d = %d\0A\00"
@.str.61052774261616141084528610567600 = private global [26 x i8] c"Shift left: %d << 2 = %d\0A\00"
@.str.56220371812088357804685438684026 = private global [27 x i8] c"Shift right: %d >> 1 = %d\0A\00"
@.str.48488151184281667861385422412878 = private global [21 x i8] c"x is greater than y\0A\00"
@.str.72540161408418624660748813725412 = private global [25 x i8] c"x is not greater than y\0A\00"
@.str.38340403450173073664612628838461 = private global [13 x i8] c"x equals 10\0A\00"
@.str.88800131358503374828622477773050 = private global [12 x i8] c"x equals 5\0A\00"
@.str.67316518686286250145211767765028 = private global [21 x i8] c"x is something else\0A\00"
@llvm.used = appending global [1 x i8*] [i8* bitcast (i64 (i8*, ...)* @printf to i8*)], section "llvm.metadata"

declare ccc i64 @printf(i8* %format, ...)

define ccc i64 @add(i64 %a, i64 %b) {
add$main:
	%0 = add i64 %a, %b
	ret i64 %0
}

define ccc i64 @sub(i64 %a, i64 %b) {
sub$main:
	%0 = sub i64 %a, %b
	ret i64 %0
}

define ccc i64 @mul(i64 %a, i64 %b) {
mul$main:
	%0 = mul i64 %a, %b
	ret i64 %0
}

define ccc i64 @div(i64 %a, i64 %b) {
div$main:
	%0 = sdiv i64 %a, %b
	ret i64 %0
}

define ccc i64 @bitwise_and(i64 %a, i64 %b) {
bitwise_and$main:
	%0 = and i64 %a, %b
	ret i64 %0
}

define ccc i64 @bitwise_or(i64 %a, i64 %b) {
bitwise_or$main:
	%0 = or i64 %a, %b
	ret i64 %0
}

define ccc i64 @shift_left(i64 %a, i64 %bits) {
shift_left$main:
	%0 = shl i64 %a, %bits
	ret i64 %0
}

define ccc i64 @shift_right(i64 %a, i64 %bits) {
shift_right$main:
	%0 = ashr i64 %a, %bits
	ret i64 %0
}

define ccc i64 @main() {
main$main:
	%x = alloca i64
	store i64 10, i64* %x
	%y = alloca i64
	store i64 3, i64* %y
	%0 = getelementptr [24 x i8], [24 x i8]* @.str.30470822845700106762304227245531, i8 0
	%1 = bitcast [24 x i8]* %0 to i8*
	%2 = load i64, i64* %x
	%3 = load i64, i64* %y
	%4 = load i64, i64* %x
	%5 = load i64, i64* %y
	%6 = call i64 @add(i64 %4, i64 %5)
	%7 = call i64 (i8*, ...) @printf(i8* %1, i64 %2, i64 %3, i64 %6)
	%8 = getelementptr [27 x i8], [27 x i8]* @.str.26764145785832063616415870381021, i8 0
	%9 = bitcast [27 x i8]* %8 to i8*
	%10 = load i64, i64* %x
	%11 = load i64, i64* %y
	%12 = load i64, i64* %x
	%13 = load i64, i64* %y
	%14 = call i64 @sub(i64 %12, i64 %13)
	%15 = call i64 (i8*, ...) @printf(i8* %9, i64 %10, i64 %11, i64 %14)
	%16 = getelementptr [30 x i8], [30 x i8]* @.str.63538466015350376607021102052383, i8 0
	%17 = bitcast [30 x i8]* %16 to i8*
	%18 = load i64, i64* %x
	%19 = load i64, i64* %y
	%20 = load i64, i64* %x
	%21 = load i64, i64* %y
	%22 = call i64 @mul(i64 %20, i64 %21)
	%23 = call i64 (i8*, ...) @printf(i8* %17, i64 %18, i64 %19, i64 %22)
	%24 = getelementptr [24 x i8], [24 x i8]* @.str.14141123841086581601010064820708, i8 0
	%25 = bitcast [24 x i8]* %24 to i8*
	%26 = load i64, i64* %x
	%27 = load i64, i64* %y
	%28 = load i64, i64* %x
	%29 = load i64, i64* %y
	%30 = call i64 @div(i64 %28, i64 %29)
	%31 = call i64 (i8*, ...) @printf(i8* %25, i64 %26, i64 %27, i64 %30)
	%32 = getelementptr [27 x i8], [27 x i8]* @.str.05026335814357373843080833558252, i8 0
	%33 = bitcast [27 x i8]* %32 to i8*
	%34 = load i64, i64* %x
	%35 = load i64, i64* %y
	%36 = load i64, i64* %x
	%37 = load i64, i64* %y
	%38 = call i64 @bitwise_and(i64 %36, i64 %37)
	%39 = call i64 (i8*, ...) @printf(i8* %33, i64 %34, i64 %35, i64 %38)
	%40 = getelementptr [26 x i8], [26 x i8]* @.str.68031407054182712361870168706145, i8 0
	%41 = bitcast [26 x i8]* %40 to i8*
	%42 = load i64, i64* %x
	%43 = load i64, i64* %y
	%44 = load i64, i64* %x
	%45 = load i64, i64* %y
	%46 = call i64 @bitwise_or(i64 %44, i64 %45)
	%47 = call i64 (i8*, ...) @printf(i8* %41, i64 %42, i64 %43, i64 %46)
	%48 = getelementptr [26 x i8], [26 x i8]* @.str.61052774261616141084528610567600, i8 0
	%49 = bitcast [26 x i8]* %48 to i8*
	%50 = load i64, i64* %x
	%51 = load i64, i64* %x
	%52 = call i64 @shift_left(i64 %51, i64 2)
	%53 = call i64 (i8*, ...) @printf(i8* %49, i64 %50, i64 %52)
	%54 = getelementptr [27 x i8], [27 x i8]* @.str.56220371812088357804685438684026, i8 0
	%55 = bitcast [27 x i8]* %54 to i8*
	%56 = load i64, i64* %x
	%57 = load i64, i64* %x
	%58 = call i64 @shift_right(i64 %57, i64 1)
	%59 = call i64 (i8*, ...) @printf(i8* %55, i64 %56, i64 %58)
	%60 = load i64, i64* %y
	%61 = load i64, i64* %x
	%62 = icmp sgt i64 %61, %60
	br i1 %62, label %if.then.1, label %if.else.1

if.then.1:
	%63 = getelementptr [21 x i8], [21 x i8]* @.str.48488151184281667861385422412878, i8 0
	%64 = bitcast [21 x i8]* %63 to i8*
	%65 = call i64 (i8*, ...) @printf(i8* %64)
	br label %if.merge.1

if.merge.1:
	%66 = load i64, i64* %x
	%67 = icmp eq i64 %66, 10
	br i1 %67, label %if.then.2, label %if.else.2

if.else.1:
	%68 = getelementptr [25 x i8], [25 x i8]* @.str.72540161408418624660748813725412, i8 0
	%69 = bitcast [25 x i8]* %68 to i8*
	%70 = call i64 (i8*, ...) @printf(i8* %69)
	br label %if.merge.1

if.then.2:
	%71 = getelementptr [13 x i8], [13 x i8]* @.str.38340403450173073664612628838461, i8 0
	%72 = bitcast [13 x i8]* %71 to i8*
	%73 = call i64 (i8*, ...) @printf(i8* %72)
	br label %if.merge.2

if.merge.2:
	ret i64 0

if.else.2:
	%74 = load i64, i64* %x
	%75 = icmp eq i64 %74, 5
	br i1 %75, label %elseif.then.3, label %elseif.else.3

elseif.then.3:
	%76 = getelementptr [12 x i8], [12 x i8]* @.str.88800131358503374828622477773050, i8 0
	%77 = bitcast [12 x i8]* %76 to i8*
	%78 = call i64 (i8*, ...) @printf(i8* %77)
	br label %if.merge.2

elseif.else.3:
	%79 = getelementptr [21 x i8], [21 x i8]* @.str.67316518686286250145211767765028, i8 0
	%80 = bitcast [21 x i8]* %79 to i8*
	%81 = call i64 (i8*, ...) @printf(i8* %80)
	br label %if.merge.2
}
