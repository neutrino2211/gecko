%sqlite3 = type opaque

@.str.56146852582251634488332535608878 = private global [9 x i8] c":memory:\00"
@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 (i8*, %sqlite3**)* @sqlite3_open to i8*)], section "llvm.metadata"

declare ccc i32 @sqlite3_open(i8* %filename, %sqlite3** %db)

define ccc i32 @main() {
main$main:
	%0 = inttoptr i64 0 to %sqlite3*
	%db = alloca %sqlite3*
	store %sqlite3* %0, %sqlite3** %db
	%1 = getelementptr [9 x i8], [9 x i8]* @.str.56146852582251634488332535608878, i8 0
	%2 = bitcast [9 x i8]* %1 to i8*
	%3 = load %sqlite3*, %sqlite3** %db
	%4 = bitcast %sqlite3* %3 to %sqlite3**
	%5 = call i32 @sqlite3_open(i8* %2, %sqlite3** %4)
	%rc = alloca i32
	store i32 %5, i32* %rc
	%6 = load i32, i32* %rc
	ret i32 %6
}
