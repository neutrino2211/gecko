%Inner = type { i32 }
%Outer = type { %Inner* }

@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define i32 @llvm_parity_method_chain_slice__Inner__double(%Inner* %self) {
llvm_parity_method_chain_slice__Inner__double$main:
	%0 = getelementptr %Inner, %Inner* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = getelementptr %Inner, %Inner* %self, i32 0, i32 0
	%3 = load i32, i32* %2
	%4 = add i32 %1, %3
	ret i32 %4
}

define i32 @llvm_parity_method_chain_slice__Outer__get_double(%Outer* %self) {
llvm_parity_method_chain_slice__Outer__get_double$main:
	%0 = getelementptr %Outer, %Outer* %self, i32 0, i32 0
	%1 = load %Inner*, %Inner** %0
	%2 = call i32 @llvm_parity_method_chain_slice__Inner__double(%Inner* %1)
	ret i32 %2
}

define ccc i32 @main() {
main$main:
	%0 = alloca %Inner
	%1 = getelementptr %Inner, %Inner* %0, i32 0, i32 0
	store i32 21, i32* %1
	%2 = load %Inner, %Inner* %0
	%inner = alloca %Inner
	store %Inner %2, %Inner* %inner
	%3 = alloca %Outer
	%4 = getelementptr %Outer, %Outer* %3, i32 0, i32 0
	store %Inner* %inner, %Inner** %4
	%5 = load %Outer, %Outer* %3
	%outer = alloca %Outer
	store %Outer %5, %Outer* %outer
	%6 = getelementptr %Outer, %Outer* %outer, i32 0, i32 0
	%7 = load %Inner*, %Inner** %6
	%8 = call i32 @llvm_parity_method_chain_slice__Inner__double(%Inner* %7)
	%9 = getelementptr %Outer, %Outer* %outer, i32 0, i32 0
	%10 = load %Inner*, %Inner** %9
	%11 = call i32 @llvm_parity_method_chain_slice__Inner__double(%Inner* %10)
	%from_expr = alloca i32
	store i32 %11, i32* %from_expr
	%12 = load i32, i32* %from_expr
	ret i32 %12
}
