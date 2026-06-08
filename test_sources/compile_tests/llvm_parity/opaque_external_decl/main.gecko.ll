%sqlite3 = type opaque

@.str.17176738514235610405401244278462 = private global [11 x i8] c"example.db\00"
@llvm.used = appending global [3 x i8*] [i8* bitcast (i32 (i8*, %sqlite3**)* @sqlite3_open to i8*), i8* bitcast (i32 (%sqlite3*)* @sqlite3_close to i8*), i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

declare ccc i32 @sqlite3_open(i8* %filename, %sqlite3** %ppDb)

declare ccc i32 @sqlite3_close(%sqlite3* %db)

define ccc i32 @ping(i8* %path) {
ping$main:
	%0 = inttoptr i64 0 to %sqlite3*
	%db = alloca %sqlite3*
	store %sqlite3* %0, %sqlite3** %db
	%1 = call i32 @sqlite3_open(i8* %path, %sqlite3** %db)
	%rc = alloca i32
	store i32 %1, i32* %rc
	%2 = load %sqlite3*, %sqlite3** %db
	%3 = bitcast %sqlite3* %2 to i8*
	%4 = icmp eq i8* %3, null
	br i1 %4, label %if.then.1, label %if.merge.1

if.then.1:
	%5 = load i32, i32* %rc
	ret i32 %5

if.merge.1:
	%6 = load %sqlite3*, %sqlite3** %db
	%7 = call i32 @sqlite3_close(%sqlite3* %6)
	ret i32 %7
}

define ccc i32 @main() {
main$main:
	%0 = getelementptr [11 x i8], [11 x i8]* @.str.17176738514235610405401244278462, i8 0
	%1 = bitcast [11 x i8]* %0 to i8*
	%2 = call i32 @ping(i8* %1)
	ret i32 %2
}
