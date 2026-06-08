%Data = type { i64 }

define ccc i64 @main() {
main$main:
	%data = alloca %Data
	%0 = getelementptr %Data, %Data* %data, i32 0, i32 0
	store i64 42, i64* %0
	%1 = inttoptr i64 0 to %Data*
	%maybe = alloca %Data*
	store %Data* %1, %Data** %maybe
	%2 = load %Data*, %Data** %maybe
	%safe = alloca %Data*
	store %Data* %2, %Data** %safe
	%3 = load %Data*, %Data** %safe
	%4 = getelementptr %Data, %Data* %3, i32 0, i32 0
	%5 = load i64, i64* %4
	ret i64 %5
}
