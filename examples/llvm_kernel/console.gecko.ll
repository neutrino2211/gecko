target triple = "arm64-apple-darwin21.4.0"

%Console = type { i16*, i64, i64, i8 }

@io_port = global i16 0
@io_byte = global i8 0
@VGA_ADDRESS = constant i64 753664
@WIDTH = constant i64 80
@HEIGHT = constant i64 25
@TAB_SIZE = constant i64 4
@CRTC_ADDR = constant i16 980
@CRTC_DATA = constant i16 981
@BLACK = constant i8 0
@BLUE = constant i8 1
@GREEN = constant i8 2
@CYAN = constant i8 3
@RED = constant i8 4
@MAGENTA = constant i8 5
@BROWN = constant i8 6
@LIGHT_GRAY = constant i8 7
@DARK_GRAY = constant i8 8
@LIGHT_BLUE = constant i8 9
@LIGHT_GREEN = constant i8 10
@LIGHT_CYAN = constant i8 11
@LIGHT_RED = constant i8 12
@LIGHT_MAGENTA = constant i8 13
@YELLOW = constant i8 14
@WHITE = constant i8 15

define ccc void @outb(i16 %port, i8 %value) {
outb$main:
	store i16 %port, i16* @io_port
	store i8 %value, i8* @io_byte
	call void asm sideeffect "movw io_port, %dx", ""()
	call void asm sideeffect "movb io_byte, %al", ""()
	call void asm sideeffect "outb %al, %dx", ""()
	ret void
}

define ccc i8 @inb(i16 %port) {
inb$main:
	store i16 %port, i16* @io_port
	call void asm sideeffect "movw io_port, %dx", ""()
	call void asm sideeffect "inb %dx, %al", ""()
	call void asm sideeffect "movb %al, io_byte", ""()
	%0 = load i8, i8* @io_byte
	ret i8 %0
}

define i8 @make_color(i8 %fg, i8 %bg) {
make_color$main:
	%0 = shl i8 %bg, 4
	%1 = or i8 %fg, %0
	ret i8 %1
}

define i16 @make_entry(i8 %c, i8 %color) {
make_entry$main:
	%0 = zext i8 %c to i16
	%1 = zext i8 %color to i16
	%2 = shl i16 %1, 8
	%3 = or i16 %0, %2
	ret i16 %3
}

define void @update_cursor(i64 %x, i64 %y) {
update_cursor$main:
	%0 = load i64, i64* @WIDTH
	%1 = mul i64 %y, %0
	%2 = add i64 %1, %x
	%3 = trunc i64 %2 to i16
	%pos = alloca i16
	store i16 %3, i16* %pos
	%4 = load i16, i16* @CRTC_ADDR
	call void @outb(i16 %4, i8 15)
	%5 = load i16, i16* @CRTC_DATA
	%6 = load i16, i16* %pos
	%7 = and i16 %6, 255
	%8 = trunc i16 %7 to i8
	call void @outb(i16 %5, i8 %8)
	%9 = load i16, i16* @CRTC_ADDR
	call void @outb(i16 %9, i8 14)
	%10 = load i16, i16* @CRTC_DATA
	%11 = load i16, i16* %pos
	%12 = ashr i16 %11, 8
	%13 = and i16 %12, 255
	%14 = trunc i16 %13 to i8
	call void @outb(i16 %10, i8 %14)
	ret void
}

define %Console @console__Console__new() {
console__Console__new$main:
	%0 = alloca %Console
	%1 = load i64, i64* @VGA_ADDRESS
	%2 = inttoptr i64 %1 to i16*
	%3 = getelementptr %Console, %Console* %0, i32 0, i32 0
	store i16* %2, i16** %3
	%4 = getelementptr %Console, %Console* %0, i32 0, i32 1
	store i64 0, i64* %4
	%5 = getelementptr %Console, %Console* %0, i32 0, i32 2
	store i64 0, i64* %5
	%6 = load i8, i8* @LIGHT_GRAY
	%7 = load i8, i8* @BLACK
	%8 = call i8 @make_color(i8 %6, i8 %7)
	%9 = getelementptr %Console, %Console* %0, i32 0, i32 3
	store i8 %8, i8* %9
	%10 = load %Console, %Console* %0
	ret %Console %10
}

define void @console__Console__set_color(%Console* %self, i8 %fg, i8 %bg) {
console__Console__set_color$main:
	%0 = getelementptr %Console, %Console* %self, i32 0, i32 3
	%1 = call i8 @make_color(i8 %fg, i8 %bg)
	store i8 %1, i8* %0
	ret void
}

define void @console__Console__clear(%Console* %self) {
console__Console__clear$main:
	%0 = getelementptr %Console, %Console* %self, i32 0, i32 3
	%1 = load i8, i8* %0
	%2 = call i16 @make_entry(i8 32, i8 %1)
	%space = alloca i16
	store i16 %2, i16* %space
	%i = alloca i64
	store i64 0, i64* %i
	br label %loop.header.1

loop.header.1:
	%3 = load i64, i64* @WIDTH
	%4 = load i64, i64* @HEIGHT
	%5 = mul i64 %3, %4
	%6 = load i64, i64* %i
	%7 = icmp slt i64 %6, %5
	br i1 %7, label %loop.body.1, label %loop.exit.1

loop.body.1:
	%8 = getelementptr %Console, %Console* %self, i32 0, i32 0
	%9 = load i64, i64* %i
	%10 = load i16, i16* %space
	%11 = load i16*, i16** %8
	%12 = getelementptr i16, i16* %11, i64 %9
	store i16 %10, i16* %12
	%13 = load i64, i64* %i
	%14 = add i64 %13, 1
	store i64 %14, i64* %i
	br label %loop.header.1

loop.exit.1:
	%15 = getelementptr %Console, %Console* %self, i32 0, i32 1
	store i64 0, i64* %15
	%16 = getelementptr %Console, %Console* %self, i32 0, i32 2
	store i64 0, i64* %16
	call void @update_cursor(i64 0, i64 0)
	ret void
}

define void @console__Console__putchar(%Console* %self, i8 %c) {
console__Console__putchar$main:
	%0 = trunc i64 10 to i8
	%1 = icmp eq i8 %c, %0
	br i1 %1, label %if.then.2, label %if.merge.2

if.then.2:
	call void @console__Console__newline(%Console* %self)
	ret void

if.merge.2:
	%2 = trunc i64 8 to i8
	%3 = icmp eq i8 %c, %2
	br i1 %3, label %if.then.3, label %if.merge.3

if.then.3:
	call void @console__Console__backspace(%Console* %self)
	ret void

if.merge.3:
	%4 = load i64, i64* @WIDTH
	%5 = getelementptr %Console, %Console* %self, i32 0, i32 1
	%6 = load i64, i64* %5
	%7 = icmp sge i64 %6, %4
	br i1 %7, label %if.then.4, label %if.merge.4

if.then.4:
	call void @console__Console__newline(%Console* %self)
	br label %if.merge.4

if.merge.4:
	%8 = getelementptr %Console, %Console* %self, i32 0, i32 2
	%9 = load i64, i64* %8
	%10 = load i64, i64* @WIDTH
	%11 = mul i64 %9, %10
	%12 = getelementptr %Console, %Console* %self, i32 0, i32 1
	%13 = load i64, i64* %12
	%14 = add i64 %11, %13
	%index = alloca i64
	store i64 %14, i64* %index
	%15 = getelementptr %Console, %Console* %self, i32 0, i32 0
	%16 = load i64, i64* %index
	%17 = getelementptr %Console, %Console* %self, i32 0, i32 3
	%18 = load i8, i8* %17
	%19 = call i16 @make_entry(i8 %c, i8 %18)
	%20 = load i16*, i16** %15
	%21 = getelementptr i16, i16* %20, i64 %16
	store i16 %19, i16* %21
	%22 = getelementptr %Console, %Console* %self, i32 0, i32 1
	%23 = getelementptr %Console, %Console* %self, i32 0, i32 1
	%24 = load i64, i64* %23
	%25 = add i64 %24, 1
	store i64 %25, i64* %22
	%26 = getelementptr %Console, %Console* %self, i32 0, i32 1
	%27 = load i64, i64* %26
	%28 = getelementptr %Console, %Console* %self, i32 0, i32 2
	%29 = load i64, i64* %28
	call void @update_cursor(i64 %27, i64 %29)
	ret void
}

define void @console__Console__newline(%Console* %self) {
console__Console__newline$main:
	%0 = getelementptr %Console, %Console* %self, i32 0, i32 1
	store i64 0, i64* %0
	%1 = getelementptr %Console, %Console* %self, i32 0, i32 2
	%2 = getelementptr %Console, %Console* %self, i32 0, i32 2
	%3 = load i64, i64* %2
	%4 = add i64 %3, 1
	store i64 %4, i64* %1
	%5 = load i64, i64* @HEIGHT
	%6 = getelementptr %Console, %Console* %self, i32 0, i32 2
	%7 = load i64, i64* %6
	%8 = icmp sge i64 %7, %5
	br i1 %8, label %if.then.5, label %if.merge.5

if.then.5:
	call void @console__Console__scroll(%Console* %self)
	%9 = getelementptr %Console, %Console* %self, i32 0, i32 2
	%10 = load i64, i64* @HEIGHT
	%11 = sub i64 %10, 1
	store i64 %11, i64* %9
	br label %if.merge.5

if.merge.5:
	%12 = getelementptr %Console, %Console* %self, i32 0, i32 1
	%13 = load i64, i64* %12
	%14 = getelementptr %Console, %Console* %self, i32 0, i32 2
	%15 = load i64, i64* %14
	call void @update_cursor(i64 %13, i64 %15)
	ret void
}

define void @console__Console__backspace(%Console* %self) {
console__Console__backspace$main:
	%0 = getelementptr %Console, %Console* %self, i32 0, i32 1
	%1 = load i64, i64* %0
	%2 = icmp sgt i64 %1, 0
	br i1 %2, label %if.then.6, label %if.merge.6

if.then.6:
	%3 = getelementptr %Console, %Console* %self, i32 0, i32 1
	%4 = getelementptr %Console, %Console* %self, i32 0, i32 1
	%5 = load i64, i64* %4
	%6 = sub i64 %5, 1
	store i64 %6, i64* %3
	%7 = getelementptr %Console, %Console* %self, i32 0, i32 2
	%8 = load i64, i64* %7
	%9 = load i64, i64* @WIDTH
	%10 = mul i64 %8, %9
	%11 = getelementptr %Console, %Console* %self, i32 0, i32 1
	%12 = load i64, i64* %11
	%13 = add i64 %10, %12
	%index = alloca i64
	store i64 %13, i64* %index
	%14 = getelementptr %Console, %Console* %self, i32 0, i32 0
	%15 = load i64, i64* %index
	%16 = getelementptr %Console, %Console* %self, i32 0, i32 3
	%17 = load i8, i8* %16
	%18 = call i16 @make_entry(i8 32, i8 %17)
	%19 = load i16*, i16** %14
	%20 = getelementptr i16, i16* %19, i64 %15
	store i16 %18, i16* %20
	%21 = getelementptr %Console, %Console* %self, i32 0, i32 1
	%22 = load i64, i64* %21
	%23 = getelementptr %Console, %Console* %self, i32 0, i32 2
	%24 = load i64, i64* %23
	call void @update_cursor(i64 %22, i64 %24)
	br label %if.merge.6

if.merge.6:
	ret void
}

define void @console__Console__scroll(%Console* %self) {
console__Console__scroll$main:
	%i = alloca i64
	store i64 0, i64* %i
	br label %loop.header.7

loop.header.7:
	%0 = load i64, i64* @WIDTH
	%1 = load i64, i64* @HEIGHT
	%2 = sub i64 %1, 1
	%3 = mul i64 %0, %2
	%4 = load i64, i64* %i
	%5 = icmp slt i64 %4, %3
	br i1 %5, label %loop.body.7, label %loop.exit.7

loop.body.7:
	%6 = getelementptr %Console, %Console* %self, i32 0, i32 0
	%7 = load i64, i64* %i
	%8 = getelementptr %Console, %Console* %self, i32 0, i32 0
	%9 = load i64, i64* %i
	%10 = load i64, i64* @WIDTH
	%11 = add i64 %9, %10
	%12 = load i16*, i16** %8
	%13 = getelementptr i16, i16* %12, i64 %11
	%14 = load i16, i16* %13
	%15 = load i16*, i16** %6
	%16 = getelementptr i16, i16* %15, i64 %7
	store i16 %14, i16* %16
	%17 = load i64, i64* %i
	%18 = add i64 %17, 1
	store i64 %18, i64* %i
	br label %loop.header.7

loop.exit.7:
	%19 = getelementptr %Console, %Console* %self, i32 0, i32 3
	%20 = load i8, i8* %19
	%21 = call i16 @make_entry(i8 32, i8 %20)
	%space = alloca i16
	store i16 %21, i16* %space
	%22 = load i64, i64* @WIDTH
	%23 = load i64, i64* @HEIGHT
	%24 = sub i64 %23, 1
	%25 = mul i64 %22, %24
	store i64 %25, i64* %i
	br label %loop.header.8

loop.header.8:
	%26 = load i64, i64* @WIDTH
	%27 = load i64, i64* @HEIGHT
	%28 = mul i64 %26, %27
	%29 = load i64, i64* %i
	%30 = icmp slt i64 %29, %28
	br i1 %30, label %loop.body.8, label %loop.exit.8

loop.body.8:
	%31 = getelementptr %Console, %Console* %self, i32 0, i32 0
	%32 = load i64, i64* %i
	%33 = load i16, i16* %space
	%34 = load i16*, i16** %31
	%35 = getelementptr i16, i16* %34, i64 %32
	store i16 %33, i16* %35
	%36 = load i64, i64* %i
	%37 = add i64 %36, 1
	store i64 %37, i64* %i
	br label %loop.header.8

loop.exit.8:
	ret void
}

define void @console__Console__print(%Console* %self, i8* %text, i64 %len) {
console__Console__print$main:
	%i = alloca i64
	store i64 0, i64* %i
	br label %loop.header.9

loop.header.9:
	%0 = load i64, i64* %i
	%1 = icmp slt i64 %0, %len
	br i1 %1, label %loop.body.9, label %loop.exit.9

loop.body.9:
	%2 = load i64, i64* %i
	%3 = getelementptr i8, i8* %text, i64 %2
	%4 = load i8, i8* %3
	call void @console__Console__putchar(%Console* %self, i8 %4)
	%5 = load i64, i64* %i
	%6 = add i64 %5, 1
	store i64 %6, i64* %i
	br label %loop.header.9

loop.exit.9:
	ret void
}

define void @console__Console__print_str(%Console* %self, i8* %s) {
console__Console__print_str$main:
	%i = alloca i64
	store i64 0, i64* %i
	br label %loop.header.10

loop.header.10:
	%0 = load i64, i64* %i
	%1 = getelementptr i8, i8* %s, i64 %0
	%2 = load i8, i8* %1
	%3 = trunc i64 0 to i8
	%4 = icmp ne i8 %2, %3
	br i1 %4, label %loop.body.10, label %loop.exit.10

loop.body.10:
	%5 = load i64, i64* %i
	%6 = getelementptr i8, i8* %s, i64 %5
	%7 = load i8, i8* %6
	call void @console__Console__putchar(%Console* %self, i8 %7)
	%8 = load i64, i64* %i
	%9 = add i64 %8, 1
	store i64 %9, i64* %i
	br label %loop.header.10

loop.exit.10:
	ret void
}
