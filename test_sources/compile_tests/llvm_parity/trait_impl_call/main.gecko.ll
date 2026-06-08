%Meter = type { i32 }

@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define i32 @Meter__Metric__score(%Meter* %self) {
Meter__Metric__score$main:
	%0 = getelementptr %Meter, %Meter* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = mul i32 %1, 2
	ret i32 %2
}

define ccc %Meter @make_meter(i32 %v) {
make_meter$main:
	%0 = alloca %Meter
	%1 = getelementptr %Meter, %Meter* %0, i32 0, i32 0
	store i32 %v, i32* %1
	%2 = load %Meter, %Meter* %0
	ret %Meter %2
}

define ccc i32 @main() {
main$main:
	%0 = call %Meter @make_meter(i32 21)
	%m = alloca %Meter
	store %Meter %0, %Meter* %m
	%1 = call i32 @Meter__Metric__score(%Meter* %m)
	%2 = call i32 @Meter__Metric__score(%Meter* %m)
	ret i32 %2
}
