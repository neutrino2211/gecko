%Probe = type { i32 }

@llvm.used = appending global [1 x i8*] [i8* bitcast (i32 ()* @main to i8*)], section "llvm.metadata"

define i32 @Probe__ChildMetric__score(%Probe* %self) {
Probe__ChildMetric__score$main:
	%0 = getelementptr %Probe, %Probe* %self, i32 0, i32 0
	%1 = load i32, i32* %0
	%2 = add i32 %1, 100
	ret i32 %2
}

define ccc %Probe @make_probe(i32 %v) {
make_probe$main:
	%0 = alloca %Probe
	%1 = getelementptr %Probe, %Probe* %0, i32 0, i32 0
	store i32 %v, i32* %1
	%2 = load %Probe, %Probe* %0
	ret %Probe %2
}

define ccc i32 @main() {
main$main:
	%0 = call %Probe @make_probe(i32 23)
	%p = alloca %Probe
	store %Probe %0, %Probe* %p
	%1 = call i32 @Probe__ChildMetric__score(%Probe* %p)
	%2 = call i32 @Probe__ChildMetric__score(%Probe* %p)
	ret i32 %2
}
