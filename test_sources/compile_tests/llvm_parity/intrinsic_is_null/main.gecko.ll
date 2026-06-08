@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define ccc i32 @maybe(i8* %ptr) {
maybe$main:
	%0 = icmp eq i8* %ptr, null
	br i1 %0, label %if.then.1, label %if.merge.1

if.then.1:
	ret i32 0

if.merge.1:
	%1 = icmp ne i8* %ptr, null
	br i1 %1, label %if.then.2, label %if.merge.2

if.then.2:
	ret i32 1

if.merge.2:
	ret i32 2
}

define ccc i32 @main() {
main$main:
	%0 = inttoptr i64 0 to i8*
	%1 = call i32 @maybe(i8* %0)
	ret i32 %1
}
