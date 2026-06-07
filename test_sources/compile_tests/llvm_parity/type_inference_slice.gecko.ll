define ccc i32 @check_inference() {
check_inference$main:
	%x = alloca i32
	store i32 41, i32* %x
	%flag = alloca i1
	store i1 true, i1* %flag
	ret i32 0
}
