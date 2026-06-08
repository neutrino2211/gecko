%sqlite3 = type opaque

@.str.51153550616042461136284453728742 = private global [9 x i8] c":memory:\00"
@llvm.used = appending global [2 x i8*] [i8* bitcast (i32 (i8*, %sqlite3**)* @sqlite3_open to i8*), i8* bitcast (i32 (%sqlite3*)* @sqlite3_close to i8*)], section "llvm.metadata"

declare ccc i32 @sqlite3_open(i8* %filename, %sqlite3** %db)

declare ccc i32 @sqlite3_close(%sqlite3* %db)

define ccc i32 @main() {
main$main:
	%0 = inttoptr i64 0 to %sqlite3*
	%db = alloca %sqlite3*
	store %sqlite3* %0, %sqlite3** %db
	%1 = getelementptr [9 x i8], [9 x i8]* @.str.51153550616042461136284453728742, i8 0
	%2 = bitcast [9 x i8]* %1 to i8*
	%3 = call i32 @sqlite3_open(i8* %2, %sqlite3** %db)
	%rc = alloca i32
	store i32 %3, i32* %rc
	%4 = load i32, i32* %rc
	%5 = trunc i64 0 to i32
	%6 = icmp ne i32 %4, %5
	br i1 %6, label %if.then.1, label %if.merge.1

if.then.1:
	%7 = load i32, i32* %rc
	ret i32 %7

if.merge.1:
	%8 = load %sqlite3*, %sqlite3** %db
	%9 = bitcast %sqlite3* %8 to i8*
	%10 = icmp eq i8* %9, null
	%11 = xor i1 %10, true
	br i1 %11, label %if.then.2, label %if.merge.2

if.then.2:
	%12 = load %sqlite3*, %sqlite3** %db
	%13 = call i32 @sqlite3_close(%sqlite3* %12)
	br label %if.merge.2

if.merge.2:
	ret i32 0
}
