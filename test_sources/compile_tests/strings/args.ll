; ModuleID = 'args.c'
source_filename = "args.c"
target datalayout = "e-m:o-i64:64-i128:128-n32:64-S128"
target triple = "arm64-apple-macosx14.0.0"

@.str = private unnamed_addr constant [6 x i8] c"Hello\00", align 1
@.str.1 = private unnamed_addr constant [4 x i8] c"One\00", align 1
@.str.2 = private unnamed_addr constant [4 x i8] c"Two\00", align 1
@.str.3 = private unnamed_addr constant [22 x i8] c"name: %s, arr[0]: %s\0A\00", align 1

; Function Attrs: noinline nounwind optnone ssp uwtable(sync)
define ptr @get_some() #0 {
  %1 = alloca ptr, align 8
  store ptr @.str, ptr %1, align 8
  %2 = load ptr, ptr %1, align 8
  ret ptr %2
}

; Function Attrs: noinline nounwind optnone ssp uwtable(sync)
define i32 @main(i32 noundef %0, ptr noundef %1) #0 {
  %3 = alloca i32, align 4
  %4 = alloca ptr, align 8
  %5 = alloca [3 x ptr], align 8
  store i32 %0, ptr %3, align 4
  store ptr %1, ptr %4, align 8
  %6 = getelementptr inbounds [3 x ptr], ptr %5, i64 0, i64 0
  store ptr @.str.1, ptr %6, align 8
  %7 = getelementptr inbounds ptr, ptr %6, i64 1
  store ptr @.str.2, ptr %7, align 8
  %8 = getelementptr inbounds ptr, ptr %7, i64 1
  %9 = call ptr @get_some()
  store ptr %9, ptr %8, align 8
  %10 = load ptr, ptr %4, align 8
  %11 = getelementptr inbounds ptr, ptr %10, i64 3
  %12 = load ptr, ptr %11, align 8
  %13 = getelementptr inbounds [3 x ptr], ptr %5, i64 0, i64 1
  %14 = load ptr, ptr %13, align 8
  %15 = call i32 (ptr, ...) @printf(ptr noundef @.str.3, ptr noundef %12, ptr noundef %14)
  ret i32 0
}

declare i32 @printf(ptr noundef, ...) #1

attributes #0 = { noinline nounwind optnone ssp uwtable(sync) "frame-pointer"="non-leaf" "no-trapping-math"="true" "stack-protector-buffer-size"="8" "target-cpu"="apple-m1" "target-features"="+aes,+crc,+crypto,+dotprod,+fp-armv8,+fp16fml,+fullfp16,+lse,+neon,+ras,+rcpc,+rdm,+sha2,+sha3,+sm4,+v8.1a,+v8.2a,+v8.3a,+v8.4a,+v8.5a,+v8a,+zcm,+zcz" }
attributes #1 = { "frame-pointer"="non-leaf" "no-trapping-math"="true" "stack-protector-buffer-size"="8" "target-cpu"="apple-m1" "target-features"="+aes,+crc,+crypto,+dotprod,+fp-armv8,+fp16fml,+fullfp16,+lse,+neon,+ras,+rcpc,+rdm,+sha2,+sha3,+sm4,+v8.1a,+v8.2a,+v8.3a,+v8.4a,+v8.5a,+v8a,+zcm,+zcz" }

!llvm.module.flags = !{!0, !1, !2, !3}
!llvm.ident = !{!4}

!0 = !{i32 1, !"wchar_size", i32 4}
!1 = !{i32 8, !"PIC Level", i32 2}
!2 = !{i32 7, !"uwtable", i32 1}
!3 = !{i32 7, !"frame-pointer", i32 1}
!4 = !{!"Homebrew clang version 16.0.6"}
