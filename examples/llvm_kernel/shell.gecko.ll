target triple = "arm64-apple-darwin21.4.0"

%Console = type { i16*, i64, i64, i8 }
%Shell = type { %Console, [256 x i8], i64, i1 }

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
@DATA_PORT = constant i16 96
@STATUS_PORT = constant i16 100
@KEY_RELEASED = constant i8 128
@MAX_INPUT = constant i64 256
@MAX_HISTORY = constant i64 8
@.str.d513be5499b21b32676eeaf066c16969 = private global [50 x i8] c"  _____ ______ _____ _  ______     ____   _____ \0A\00"
@.str.e904e26b461ea78610ea0d393210dd64 = private global [50 x i8] c" / ____|  ____/ ____| |/ / __ \5C   / __ \5C / ____|\0A\00"
@.str.b93b1a1850acdecab29c3d5c52ccf29f = private global [50 x i8] c"| |  __| |__ | |    | ' / |  | | | |  | | (___  \0A\00"
@.str.dccf8a59eeeb2e5d25657e7890a39c85 = private global [50 x i8] c"| | |_ |  __|| |    |  <| |  | | | |  | |\5C___ \5C \0A\00"
@.str.de027d4483f9de47e6f60d9373d11074 = private global [50 x i8] c"| |__| | |___| |____| . \5C |__| | | |__| |____) |\0A\00"
@.str.e50640ed7c4f6cca453cb7940fcb6f3a = private global [50 x i8] c" \5C_____|______\5C_____|_|\5C_\5C____/   \5C____/|_____/ \0A\00"
@.str.877b3b1df662f115c58ae79b95e210cd = private global [2 x i8] c"\0A\00"
@.str.7a48544064398318a1a1a1ace5d1e0b1 = private global [63 x i8] c"LLVM Gecko OS v0.1 - A kernel written in Gecko (LLVM Backend)\0A\00"
@.str.6245fa9a58695cf6290d102c8e057370 = private global [38 x i8] c"Type 'help' for available commands.\0A\0A\00"
@.str.eea52f6d2b437d3872be303e99443816 = private global [6 x i8] c"gecko\00"
@.str.39f0d120479bb2b6f1535d0412cd4968 = private global [3 x i8] c"> \00"
@.str.9f3927d0e163baf5539ea37886535aaa = private global [5 x i8] c"help\00"
@.str.284764937a0a1705e0e2e30e8d058982 = private global [6 x i8] c"clear\00"
@.str.f784d5eded00ed2c54ae36fa4623042c = private global [6 x i8] c"about\00"
@.str.c19502b53bd3a77485327307117edc86 = private global [5 x i8] c"echo\00"
@.str.bb5a95bc7f7046905b337e6d0913cbca = private global [6 x i8] c"color\00"
@.str.1550e026e8352d233206021ac4d6d521 = private global [7 x i8] c"reboot\00"
@.str.f86148bdcacfd7429957accf12acfa84 = private global [5 x i8] c"halt\00"
@.str.e27b4567063441d6c834aa85f74729b4 = private global [18 x i8] c"Unknown command: \00"
@.str.2ba011ef04817ef47d8f37f41e9e926f = private global [2 x i8] c"\0A\00"
@.str.2c3ab4e8b3f8b9bd414eb2d9fadf6461 = private global [21 x i8] c"Available commands:\0A\00"
@.str.b340f932650ad298ca39a0b566d0dceb = private global [35 x i8] c"  help   - Show this help message\0A\00"
@.str.c3e61d2868ac776a1d01854b78a64b7b = private global [29 x i8] c"  clear  - Clear the screen\0A\00"
@.str.856d9ce6d8d3e907ac33c104f1bc47b8 = private global [27 x i8] c"  about  - About Gecko OS\0A\00"
@.str.903326f55aa7bf0f5993fd9e58790710 = private global [27 x i8] c"  echo   - Echo text back\0A\00"
@.str.9873538ffc94151a9114f5021d150737 = private global [30 x i8] c"  color  - Change text color\0A\00"
@.str.17c569c7521478aa2070f405ccc51093 = private global [30 x i8] c"  reboot - Reboot the system\0A\00"
@.str.985bcffd0a6afd25a5be032986a30bf6 = private global [25 x i8] c"  halt   - Halt the CPU\0A\00"
@.str.d19e1f2dc0f9f01aba007f4665999260 = private global [16 x i8] c"\0AGecko OS v0.1\0A\00"
@.str.8987c9f6edab3347f7705e66f961158e = private global [46 x i8] c"A minimal kernel written entirely in Gecko.\0A\0A\00"
@.str.59d4e94a1653ab2861ef47e3caeb580a = private global [11 x i8] c"Features:\0A\00"
@.str.db4050860aa28162356f27aa713423cc = private global [42 x i8] c"  - VGA text mode console with scrolling\0A\00"
@.str.07e46d667a50bcacee8f890f1c573d52 = private global [35 x i8] c"  - PS/2 keyboard input (polling)\0A\00"
@.str.bde5c99d7bb57c1815245963f27df98c = private global [27 x i8] c"  - Simple command shell\0A\0A\00"
@.str.2a95262bdf56446a55105b4f08635b7c = private global [13 x i8] c"Built with:\0A\00"
@.str.75b2256a66d831e688f10ff8b2321da0 = private global [32 x i8] c"  - Gecko programming language\0A\00"
@.str.531ed4c67e1ae4bd108543997eb0f3d8 = private global [33 x i8] c"  - LLVM Backend (llc + ld.lld)\0A\00"
@.str.2771e045e897b27450254a8c3af506ba = private global [21 x i8] c"  - Module imports\0A\0A\00"
@.str.a5ef6a3809d4a7744e1a3cd3955d64bb = private global [21 x i8] c"Color set to green!\0A\00"
@.str.7a2cdb4d39f68772427d175ed39f6c95 = private global [14 x i8] c"Rebooting...\0A\00"
@.str.0319bb33ef05673833765b1bc4214e62 = private global [23 x i8] c"Halting CPU. Goodbye!\0A\00"

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

define ccc i1 @has_key() {
has_key$main:
	%0 = load i16, i16* @STATUS_PORT
	%1 = call i8 @inb(i16 %0)
	%status = alloca i8
	store i8 %1, i8* %status
	%2 = load i8, i8* %status
	%3 = and i8 %2, 1
	%4 = trunc i64 0 to i8
	%5 = icmp ne i8 %3, %4
	ret i1 %5
}

define ccc i8 @read_scancode() {
read_scancode$main:
	%0 = load i16, i16* @DATA_PORT
	%1 = call i8 @inb(i16 %0)
	ret i8 %1
}

define ccc i8 @scancode_to_char(i8 %scancode) {
scancode_to_char$main:
	%0 = load i8, i8* @KEY_RELEASED
	%1 = icmp sge i8 %scancode, %0
	br i1 %1, label %if.then.11, label %if.merge.11

if.then.11:
	ret i8 0

if.merge.11:
	%2 = trunc i64 28 to i8
	%3 = icmp eq i8 %scancode, %2
	br i1 %3, label %if.then.12, label %if.merge.12

if.then.12:
	ret i8 10

if.merge.12:
	%4 = trunc i64 14 to i8
	%5 = icmp eq i8 %scancode, %4
	br i1 %5, label %if.then.13, label %if.merge.13

if.then.13:
	ret i8 8

if.merge.13:
	%6 = trunc i64 57 to i8
	%7 = icmp eq i8 %scancode, %6
	br i1 %7, label %if.then.14, label %if.merge.14

if.then.14:
	ret i8 32

if.merge.14:
	%8 = trunc i64 2 to i8
	%9 = icmp eq i8 %scancode, %8
	br i1 %9, label %if.then.15, label %if.merge.15

if.then.15:
	ret i8 49

if.merge.15:
	%10 = trunc i64 3 to i8
	%11 = icmp eq i8 %scancode, %10
	br i1 %11, label %if.then.16, label %if.merge.16

if.then.16:
	ret i8 50

if.merge.16:
	%12 = trunc i64 4 to i8
	%13 = icmp eq i8 %scancode, %12
	br i1 %13, label %if.then.17, label %if.merge.17

if.then.17:
	ret i8 51

if.merge.17:
	%14 = trunc i64 5 to i8
	%15 = icmp eq i8 %scancode, %14
	br i1 %15, label %if.then.18, label %if.merge.18

if.then.18:
	ret i8 52

if.merge.18:
	%16 = trunc i64 6 to i8
	%17 = icmp eq i8 %scancode, %16
	br i1 %17, label %if.then.19, label %if.merge.19

if.then.19:
	ret i8 53

if.merge.19:
	%18 = trunc i64 7 to i8
	%19 = icmp eq i8 %scancode, %18
	br i1 %19, label %if.then.20, label %if.merge.20

if.then.20:
	ret i8 54

if.merge.20:
	%20 = trunc i64 8 to i8
	%21 = icmp eq i8 %scancode, %20
	br i1 %21, label %if.then.21, label %if.merge.21

if.then.21:
	ret i8 55

if.merge.21:
	%22 = trunc i64 9 to i8
	%23 = icmp eq i8 %scancode, %22
	br i1 %23, label %if.then.22, label %if.merge.22

if.then.22:
	ret i8 56

if.merge.22:
	%24 = trunc i64 10 to i8
	%25 = icmp eq i8 %scancode, %24
	br i1 %25, label %if.then.23, label %if.merge.23

if.then.23:
	ret i8 57

if.merge.23:
	%26 = trunc i64 11 to i8
	%27 = icmp eq i8 %scancode, %26
	br i1 %27, label %if.then.24, label %if.merge.24

if.then.24:
	ret i8 48

if.merge.24:
	%28 = trunc i64 16 to i8
	%29 = icmp eq i8 %scancode, %28
	br i1 %29, label %if.then.25, label %if.merge.25

if.then.25:
	ret i8 113

if.merge.25:
	%30 = trunc i64 17 to i8
	%31 = icmp eq i8 %scancode, %30
	br i1 %31, label %if.then.26, label %if.merge.26

if.then.26:
	ret i8 119

if.merge.26:
	%32 = trunc i64 18 to i8
	%33 = icmp eq i8 %scancode, %32
	br i1 %33, label %if.then.27, label %if.merge.27

if.then.27:
	ret i8 101

if.merge.27:
	%34 = trunc i64 19 to i8
	%35 = icmp eq i8 %scancode, %34
	br i1 %35, label %if.then.28, label %if.merge.28

if.then.28:
	ret i8 114

if.merge.28:
	%36 = trunc i64 20 to i8
	%37 = icmp eq i8 %scancode, %36
	br i1 %37, label %if.then.29, label %if.merge.29

if.then.29:
	ret i8 116

if.merge.29:
	%38 = trunc i64 21 to i8
	%39 = icmp eq i8 %scancode, %38
	br i1 %39, label %if.then.30, label %if.merge.30

if.then.30:
	ret i8 121

if.merge.30:
	%40 = trunc i64 22 to i8
	%41 = icmp eq i8 %scancode, %40
	br i1 %41, label %if.then.31, label %if.merge.31

if.then.31:
	ret i8 117

if.merge.31:
	%42 = trunc i64 23 to i8
	%43 = icmp eq i8 %scancode, %42
	br i1 %43, label %if.then.32, label %if.merge.32

if.then.32:
	ret i8 105

if.merge.32:
	%44 = trunc i64 24 to i8
	%45 = icmp eq i8 %scancode, %44
	br i1 %45, label %if.then.33, label %if.merge.33

if.then.33:
	ret i8 111

if.merge.33:
	%46 = trunc i64 25 to i8
	%47 = icmp eq i8 %scancode, %46
	br i1 %47, label %if.then.34, label %if.merge.34

if.then.34:
	ret i8 112

if.merge.34:
	%48 = trunc i64 30 to i8
	%49 = icmp eq i8 %scancode, %48
	br i1 %49, label %if.then.35, label %if.merge.35

if.then.35:
	ret i8 97

if.merge.35:
	%50 = trunc i64 31 to i8
	%51 = icmp eq i8 %scancode, %50
	br i1 %51, label %if.then.36, label %if.merge.36

if.then.36:
	ret i8 115

if.merge.36:
	%52 = trunc i64 32 to i8
	%53 = icmp eq i8 %scancode, %52
	br i1 %53, label %if.then.37, label %if.merge.37

if.then.37:
	ret i8 100

if.merge.37:
	%54 = trunc i64 33 to i8
	%55 = icmp eq i8 %scancode, %54
	br i1 %55, label %if.then.38, label %if.merge.38

if.then.38:
	ret i8 102

if.merge.38:
	%56 = trunc i64 34 to i8
	%57 = icmp eq i8 %scancode, %56
	br i1 %57, label %if.then.39, label %if.merge.39

if.then.39:
	ret i8 103

if.merge.39:
	%58 = trunc i64 35 to i8
	%59 = icmp eq i8 %scancode, %58
	br i1 %59, label %if.then.40, label %if.merge.40

if.then.40:
	ret i8 104

if.merge.40:
	%60 = trunc i64 36 to i8
	%61 = icmp eq i8 %scancode, %60
	br i1 %61, label %if.then.41, label %if.merge.41

if.then.41:
	ret i8 106

if.merge.41:
	%62 = trunc i64 37 to i8
	%63 = icmp eq i8 %scancode, %62
	br i1 %63, label %if.then.42, label %if.merge.42

if.then.42:
	ret i8 107

if.merge.42:
	%64 = trunc i64 38 to i8
	%65 = icmp eq i8 %scancode, %64
	br i1 %65, label %if.then.43, label %if.merge.43

if.then.43:
	ret i8 108

if.merge.43:
	%66 = trunc i64 44 to i8
	%67 = icmp eq i8 %scancode, %66
	br i1 %67, label %if.then.44, label %if.merge.44

if.then.44:
	ret i8 122

if.merge.44:
	%68 = trunc i64 45 to i8
	%69 = icmp eq i8 %scancode, %68
	br i1 %69, label %if.then.45, label %if.merge.45

if.then.45:
	ret i8 120

if.merge.45:
	%70 = trunc i64 46 to i8
	%71 = icmp eq i8 %scancode, %70
	br i1 %71, label %if.then.46, label %if.merge.46

if.then.46:
	ret i8 99

if.merge.46:
	%72 = trunc i64 47 to i8
	%73 = icmp eq i8 %scancode, %72
	br i1 %73, label %if.then.47, label %if.merge.47

if.then.47:
	ret i8 118

if.merge.47:
	%74 = trunc i64 48 to i8
	%75 = icmp eq i8 %scancode, %74
	br i1 %75, label %if.then.48, label %if.merge.48

if.then.48:
	ret i8 98

if.merge.48:
	%76 = trunc i64 49 to i8
	%77 = icmp eq i8 %scancode, %76
	br i1 %77, label %if.then.49, label %if.merge.49

if.then.49:
	ret i8 110

if.merge.49:
	%78 = trunc i64 50 to i8
	%79 = icmp eq i8 %scancode, %78
	br i1 %79, label %if.then.50, label %if.merge.50

if.then.50:
	ret i8 109

if.merge.50:
	%80 = trunc i64 12 to i8
	%81 = icmp eq i8 %scancode, %80
	br i1 %81, label %if.then.51, label %if.merge.51

if.then.51:
	ret i8 45

if.merge.51:
	%82 = trunc i64 13 to i8
	%83 = icmp eq i8 %scancode, %82
	br i1 %83, label %if.then.52, label %if.merge.52

if.then.52:
	ret i8 61

if.merge.52:
	%84 = trunc i64 26 to i8
	%85 = icmp eq i8 %scancode, %84
	br i1 %85, label %if.then.53, label %if.merge.53

if.then.53:
	ret i8 91

if.merge.53:
	%86 = trunc i64 27 to i8
	%87 = icmp eq i8 %scancode, %86
	br i1 %87, label %if.then.54, label %if.merge.54

if.then.54:
	ret i8 93

if.merge.54:
	%88 = trunc i64 39 to i8
	%89 = icmp eq i8 %scancode, %88
	br i1 %89, label %if.then.55, label %if.merge.55

if.then.55:
	ret i8 59

if.merge.55:
	%90 = trunc i64 40 to i8
	%91 = icmp eq i8 %scancode, %90
	br i1 %91, label %if.then.56, label %if.merge.56

if.then.56:
	ret i8 39

if.merge.56:
	%92 = trunc i64 51 to i8
	%93 = icmp eq i8 %scancode, %92
	br i1 %93, label %if.then.57, label %if.merge.57

if.then.57:
	ret i8 44

if.merge.57:
	%94 = trunc i64 52 to i8
	%95 = icmp eq i8 %scancode, %94
	br i1 %95, label %if.then.58, label %if.merge.58

if.then.58:
	ret i8 46

if.merge.58:
	%96 = trunc i64 53 to i8
	%97 = icmp eq i8 %scancode, %96
	br i1 %97, label %if.then.59, label %if.merge.59

if.then.59:
	ret i8 47

if.merge.59:
	ret i8 0
}

define ccc i8 @poll_char() {
poll_char$main:
	%0 = call i1 @has_key()
	br i1 %0, label %if.then.60, label %if.merge.60

if.then.60:
	%1 = call i8 @read_scancode()
	%sc = alloca i8
	store i8 %1, i8* %sc
	%2 = load i8, i8* %sc
	%3 = call i8 @scancode_to_char(i8 %2)
	ret i8 %3

if.merge.60:
	ret i8 0
}

define i8 @wait_char() {
wait_char$main:
	%c = alloca i8
	store i8 0, i8* %c
	br label %loop.header.61

loop.header.61:
	%0 = load i8, i8* %c
	%1 = trunc i64 0 to i8
	%2 = icmp eq i8 %0, %1
	br i1 %2, label %loop.body.61, label %loop.exit.61

loop.body.61:
	%3 = call i8 @poll_char()
	store i8 %3, i8* %c
	br label %loop.header.61

loop.exit.61:
	%4 = load i8, i8* %c
	ret i8 %4
}

define %Shell @shell__Shell__new(%Console %con) {
shell__Shell__new$main:
	%0 = alloca %Shell
	%1 = getelementptr %Shell, %Shell* %0, i32 0, i32 0
	store %Console %con, %Console* %1
	%2 = getelementptr %Shell, %Shell* %0, i32 0, i32 2
	store i64 0, i64* %2
	%3 = getelementptr %Shell, %Shell* %0, i32 0, i32 3
	store i1 true, i1* %3
	%4 = load %Shell, %Shell* %0
	%sh = alloca %Shell
	store %Shell %4, %Shell* %sh
	%5 = load %Shell, %Shell* %sh
	ret %Shell %5
}

define void @shell__Shell__run(%Shell* %self) {
shell__Shell__run$main:
	call void @shell__Shell__print_banner(%Shell* %self)
	br label %loop.header.62

loop.header.62:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 3
	%1 = load i1, i1* %0
	br i1 %1, label %loop.body.62, label %loop.exit.62

loop.body.62:
	call void @shell__Shell__print_prompt(%Shell* %self)
	call void @shell__Shell__read_line(%Shell* %self)
	call void @shell__Shell__execute(%Shell* %self)
	br label %loop.header.62

loop.exit.62:
	ret void
}

define void @shell__Shell__print_banner(%Shell* %self) {
shell__Shell__print_banner$main:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%1 = load i8, i8* @LIGHT_CYAN
	%2 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %0, i8 %1, i8 %2)
	%3 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%4 = getelementptr [50 x i8], [50 x i8]* @.str.d513be5499b21b32676eeaf066c16969, i8 0
	%5 = bitcast [50 x i8]* %4 to i8*
	call void @console__Console__print_str(%Console* %3, i8* %5)
	%6 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%7 = getelementptr [50 x i8], [50 x i8]* @.str.e904e26b461ea78610ea0d393210dd64, i8 0
	%8 = bitcast [50 x i8]* %7 to i8*
	call void @console__Console__print_str(%Console* %6, i8* %8)
	%9 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%10 = getelementptr [50 x i8], [50 x i8]* @.str.b93b1a1850acdecab29c3d5c52ccf29f, i8 0
	%11 = bitcast [50 x i8]* %10 to i8*
	call void @console__Console__print_str(%Console* %9, i8* %11)
	%12 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%13 = getelementptr [50 x i8], [50 x i8]* @.str.dccf8a59eeeb2e5d25657e7890a39c85, i8 0
	%14 = bitcast [50 x i8]* %13 to i8*
	call void @console__Console__print_str(%Console* %12, i8* %14)
	%15 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%16 = getelementptr [50 x i8], [50 x i8]* @.str.de027d4483f9de47e6f60d9373d11074, i8 0
	%17 = bitcast [50 x i8]* %16 to i8*
	call void @console__Console__print_str(%Console* %15, i8* %17)
	%18 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%19 = getelementptr [50 x i8], [50 x i8]* @.str.e50640ed7c4f6cca453cb7940fcb6f3a, i8 0
	%20 = bitcast [50 x i8]* %19 to i8*
	call void @console__Console__print_str(%Console* %18, i8* %20)
	%21 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%22 = getelementptr [2 x i8], [2 x i8]* @.str.877b3b1df662f115c58ae79b95e210cd, i8 0
	%23 = bitcast [2 x i8]* %22 to i8*
	call void @console__Console__print_str(%Console* %21, i8* %23)
	%24 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%25 = load i8, i8* @LIGHT_GRAY
	%26 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %24, i8 %25, i8 %26)
	%27 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%28 = getelementptr [63 x i8], [63 x i8]* @.str.7a48544064398318a1a1a1ace5d1e0b1, i8 0
	%29 = bitcast [63 x i8]* %28 to i8*
	call void @console__Console__print_str(%Console* %27, i8* %29)
	%30 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%31 = getelementptr [38 x i8], [38 x i8]* @.str.6245fa9a58695cf6290d102c8e057370, i8 0
	%32 = bitcast [38 x i8]* %31 to i8*
	call void @console__Console__print_str(%Console* %30, i8* %32)
	ret void
}

define void @shell__Shell__print_prompt(%Shell* %self) {
shell__Shell__print_prompt$main:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%1 = load i8, i8* @LIGHT_GREEN
	%2 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %0, i8 %1, i8 %2)
	%3 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%4 = getelementptr [6 x i8], [6 x i8]* @.str.eea52f6d2b437d3872be303e99443816, i8 0
	%5 = bitcast [6 x i8]* %4 to i8*
	call void @console__Console__print_str(%Console* %3, i8* %5)
	%6 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%7 = load i8, i8* @WHITE
	%8 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %6, i8 %7, i8 %8)
	%9 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%10 = getelementptr [3 x i8], [3 x i8]* @.str.39f0d120479bb2b6f1535d0412cd4968, i8 0
	%11 = bitcast [3 x i8]* %10 to i8*
	call void @console__Console__print_str(%Console* %9, i8* %11)
	%12 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%13 = load i8, i8* @LIGHT_GRAY
	%14 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %12, i8 %13, i8 %14)
	ret void
}

define void @shell__Shell__read_line(%Shell* %self) {
shell__Shell__read_line$main:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	store i64 0, i64* %0
	%i = alloca i64
	store i64 0, i64* %i
	br label %loop.header.63

loop.header.63:
	%1 = load i64, i64* @MAX_INPUT
	%2 = load i64, i64* %i
	%3 = icmp slt i64 %2, %1
	br i1 %3, label %loop.body.63, label %loop.exit.63

loop.body.63:
	%4 = getelementptr %Shell, %Shell* %self, i32 0, i32 1
	%5 = load i64, i64* %i
	%6 = getelementptr [256 x i8], [256 x i8]* %4, i64 0, i64 %5
	store i8 0, i8* %6
	%7 = load i64, i64* %i
	%8 = add i64 %7, 1
	store i64 %8, i64* %i
	br label %loop.header.63

loop.exit.63:
	%done = alloca i1
	store i1 false, i1* %done
	br label %loop.header.64

loop.header.64:
	%9 = load i1, i1* %done
	%10 = xor i1 %9, true
	br i1 %10, label %loop.body.64, label %loop.exit.64

loop.body.64:
	%11 = call i8 @wait_char()
	%c = alloca i8
	store i8 %11, i8* %c
	%12 = load i8, i8* %c
	%13 = trunc i64 10 to i8
	%14 = icmp eq i8 %12, %13
	br i1 %14, label %if.then.65, label %if.else.65

loop.exit.64:
	ret void

if.then.65:
	%15 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	call void @console__Console__putchar(%Console* %15, i8 10)
	store i1 true, i1* %done
	br label %if.merge.65

if.merge.65:
	br label %loop.header.64

if.else.65:
	%16 = load i8, i8* %c
	%17 = trunc i64 8 to i8
	%18 = icmp eq i8 %16, %17
	br i1 %18, label %elseif.then.66, label %elseif.else.66

elseif.then.66:
	%19 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%20 = load i64, i64* %19
	%21 = icmp sgt i64 %20, 0
	br i1 %21, label %if.then.67, label %if.merge.67

elseif.else.66:
	%22 = load i8, i8* %c
	%23 = trunc i64 0 to i8
	%24 = icmp ne i8 %22, %23
	br i1 %24, label %elseif.then.68, label %if.merge.65

if.then.67:
	%25 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%26 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%27 = load i64, i64* %26
	%28 = sub i64 %27, 1
	store i64 %28, i64* %25
	%29 = getelementptr %Shell, %Shell* %self, i32 0, i32 1
	%30 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%31 = load i64, i64* %30
	%32 = getelementptr [256 x i8], [256 x i8]* %29, i64 0, i64 %31
	store i8 0, i8* %32
	%33 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	call void @console__Console__putchar(%Console* %33, i8 8)
	br label %if.merge.67

if.merge.67:
	br label %if.merge.65

elseif.then.68:
	%34 = load i64, i64* @MAX_INPUT
	%35 = sub i64 %34, 1
	%36 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%37 = load i64, i64* %36
	%38 = icmp slt i64 %37, %35
	br i1 %38, label %if.then.69, label %if.merge.69

if.then.69:
	%39 = getelementptr %Shell, %Shell* %self, i32 0, i32 1
	%40 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%41 = load i64, i64* %40
	%42 = load i8, i8* %c
	%43 = getelementptr [256 x i8], [256 x i8]* %39, i64 0, i64 %41
	store i8 %42, i8* %43
	%44 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%45 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%46 = load i64, i64* %45
	%47 = add i64 %46, 1
	store i64 %47, i64* %44
	%48 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%49 = load i8, i8* %c
	call void @console__Console__putchar(%Console* %48, i8 %49)
	br label %if.merge.69

if.merge.69:
	br label %if.merge.65
}

define void @shell__Shell__execute(%Shell* %self) {
shell__Shell__execute$main:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%1 = load i64, i64* %0
	%2 = icmp eq i64 %1, 0
	br i1 %2, label %if.then.70, label %if.merge.70

if.then.70:
	ret void

if.merge.70:
	%3 = getelementptr [5 x i8], [5 x i8]* @.str.9f3927d0e163baf5539ea37886535aaa, i8 0
	%4 = call i1 @shell__Shell__str_eq(%Shell* %self, [5 x i8]* %3)
	br i1 %4, label %if.then.71, label %if.else.71

if.then.71:
	call void @shell__Shell__cmd_help(%Shell* %self)
	br label %if.merge.71

if.merge.71:
	ret void

if.else.71:
	%5 = getelementptr [6 x i8], [6 x i8]* @.str.284764937a0a1705e0e2e30e8d058982, i8 0
	%6 = call i1 @shell__Shell__str_eq(%Shell* %self, [6 x i8]* %5)
	br i1 %6, label %elseif.then.72, label %elseif.else.72

elseif.then.72:
	call void @shell__Shell__cmd_clear(%Shell* %self)
	br label %if.merge.71

elseif.else.72:
	%7 = getelementptr [6 x i8], [6 x i8]* @.str.f784d5eded00ed2c54ae36fa4623042c, i8 0
	%8 = call i1 @shell__Shell__str_eq(%Shell* %self, [6 x i8]* %7)
	br i1 %8, label %elseif.then.73, label %elseif.else.73

elseif.then.73:
	call void @shell__Shell__cmd_about(%Shell* %self)
	br label %if.merge.71

elseif.else.73:
	%9 = getelementptr [5 x i8], [5 x i8]* @.str.c19502b53bd3a77485327307117edc86, i8 0
	%10 = call i1 @shell__Shell__str_eq(%Shell* %self, [5 x i8]* %9)
	br i1 %10, label %elseif.then.74, label %elseif.else.74

elseif.then.74:
	call void @shell__Shell__cmd_echo(%Shell* %self)
	br label %if.merge.71

elseif.else.74:
	%11 = getelementptr [6 x i8], [6 x i8]* @.str.bb5a95bc7f7046905b337e6d0913cbca, i8 0
	%12 = call i1 @shell__Shell__str_eq(%Shell* %self, [6 x i8]* %11)
	br i1 %12, label %elseif.then.75, label %elseif.else.75

elseif.then.75:
	call void @shell__Shell__cmd_color(%Shell* %self)
	br label %if.merge.71

elseif.else.75:
	%13 = getelementptr [7 x i8], [7 x i8]* @.str.1550e026e8352d233206021ac4d6d521, i8 0
	%14 = call i1 @shell__Shell__str_eq(%Shell* %self, [7 x i8]* %13)
	br i1 %14, label %elseif.then.76, label %elseif.else.76

elseif.then.76:
	call void @shell__Shell__cmd_reboot(%Shell* %self)
	br label %if.merge.71

elseif.else.76:
	%15 = getelementptr [5 x i8], [5 x i8]* @.str.f86148bdcacfd7429957accf12acfa84, i8 0
	%16 = call i1 @shell__Shell__str_eq(%Shell* %self, [5 x i8]* %15)
	br i1 %16, label %elseif.then.77, label %elseif.else.77

elseif.then.77:
	call void @shell__Shell__cmd_halt(%Shell* %self)
	br label %if.merge.71

elseif.else.77:
	%17 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%18 = load i8, i8* @LIGHT_RED
	%19 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %17, i8 %18, i8 %19)
	%20 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%21 = getelementptr [18 x i8], [18 x i8]* @.str.e27b4567063441d6c834aa85f74729b4, i8 0
	%22 = bitcast [18 x i8]* %21 to i8*
	call void @console__Console__print_str(%Console* %20, i8* %22)
	%23 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%24 = getelementptr %Shell, %Shell* %self, i32 0, i32 1
	%25 = getelementptr [256 x i8], [256 x i8]* %24, i64 0, i64 0
	%26 = load i8, i8* %25
	%27 = inttoptr i8 %26 to i8*
	%28 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%29 = load i64, i64* %28
	call void @console__Console__print(%Console* %23, i8* %27, i64 %29)
	%30 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%31 = getelementptr [2 x i8], [2 x i8]* @.str.2ba011ef04817ef47d8f37f41e9e926f, i8 0
	%32 = bitcast [2 x i8]* %31 to i8*
	call void @console__Console__print_str(%Console* %30, i8* %32)
	%33 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%34 = load i8, i8* @LIGHT_GRAY
	%35 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %33, i8 %34, i8 %35)
	br label %if.merge.71
}

define i1 @shell__Shell__str_eq(%Shell* %self, i8* %cmd) {
shell__Shell__str_eq$main:
	%i = alloca i64
	store i64 0, i64* %i
	br label %loop.header.78

loop.header.78:
	%0 = load i64, i64* %i
	%1 = getelementptr i8, i8* %cmd, i64 %0
	%2 = load i8, i8* %1
	%3 = trunc i64 0 to i8
	%4 = icmp ne i8 %2, %3
	br i1 %4, label %loop.body.78, label %loop.exit.78

loop.body.78:
	%5 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%6 = load i64, i64* %5
	%7 = load i64, i64* %i
	%8 = icmp sge i64 %7, %6
	br i1 %8, label %if.then.79, label %if.merge.79

loop.exit.78:
	%9 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%10 = load i64, i64* %9
	%11 = load i64, i64* %i
	%12 = icmp eq i64 %11, %10
	br i1 %12, label %if.then.81, label %if.merge.81

if.then.79:
	ret i1 false

if.merge.79:
	%13 = load i64, i64* %i
	%14 = getelementptr i8, i8* %cmd, i64 %13
	%15 = load i8, i8* %14
	%16 = getelementptr %Shell, %Shell* %self, i32 0, i32 1
	%17 = load i64, i64* %i
	%18 = getelementptr [256 x i8], [256 x i8]* %16, i64 0, i64 %17
	%19 = load i8, i8* %18
	%20 = icmp ne i8 %19, %15
	br i1 %20, label %if.then.80, label %if.merge.80

if.then.80:
	ret i1 false

if.merge.80:
	%21 = load i64, i64* %i
	%22 = add i64 %21, 1
	store i64 %22, i64* %i
	br label %loop.header.78

if.then.81:
	ret i1 true

if.merge.81:
	%23 = getelementptr %Shell, %Shell* %self, i32 0, i32 1
	%24 = load i64, i64* %i
	%25 = getelementptr [256 x i8], [256 x i8]* %23, i64 0, i64 %24
	%26 = load i8, i8* %25
	%27 = trunc i64 32 to i8
	%28 = icmp eq i8 %26, %27
	ret i1 %28
}

define void @shell__Shell__cmd_help(%Shell* %self) {
shell__Shell__cmd_help$main:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%1 = load i8, i8* @YELLOW
	%2 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %0, i8 %1, i8 %2)
	%3 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%4 = getelementptr [21 x i8], [21 x i8]* @.str.2c3ab4e8b3f8b9bd414eb2d9fadf6461, i8 0
	%5 = bitcast [21 x i8]* %4 to i8*
	call void @console__Console__print_str(%Console* %3, i8* %5)
	%6 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%7 = load i8, i8* @LIGHT_GRAY
	%8 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %6, i8 %7, i8 %8)
	%9 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%10 = getelementptr [35 x i8], [35 x i8]* @.str.b340f932650ad298ca39a0b566d0dceb, i8 0
	%11 = bitcast [35 x i8]* %10 to i8*
	call void @console__Console__print_str(%Console* %9, i8* %11)
	%12 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%13 = getelementptr [29 x i8], [29 x i8]* @.str.c3e61d2868ac776a1d01854b78a64b7b, i8 0
	%14 = bitcast [29 x i8]* %13 to i8*
	call void @console__Console__print_str(%Console* %12, i8* %14)
	%15 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%16 = getelementptr [27 x i8], [27 x i8]* @.str.856d9ce6d8d3e907ac33c104f1bc47b8, i8 0
	%17 = bitcast [27 x i8]* %16 to i8*
	call void @console__Console__print_str(%Console* %15, i8* %17)
	%18 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%19 = getelementptr [27 x i8], [27 x i8]* @.str.903326f55aa7bf0f5993fd9e58790710, i8 0
	%20 = bitcast [27 x i8]* %19 to i8*
	call void @console__Console__print_str(%Console* %18, i8* %20)
	%21 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%22 = getelementptr [30 x i8], [30 x i8]* @.str.9873538ffc94151a9114f5021d150737, i8 0
	%23 = bitcast [30 x i8]* %22 to i8*
	call void @console__Console__print_str(%Console* %21, i8* %23)
	%24 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%25 = getelementptr [30 x i8], [30 x i8]* @.str.17c569c7521478aa2070f405ccc51093, i8 0
	%26 = bitcast [30 x i8]* %25 to i8*
	call void @console__Console__print_str(%Console* %24, i8* %26)
	%27 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%28 = getelementptr [25 x i8], [25 x i8]* @.str.985bcffd0a6afd25a5be032986a30bf6, i8 0
	%29 = bitcast [25 x i8]* %28 to i8*
	call void @console__Console__print_str(%Console* %27, i8* %29)
	ret void
}

define void @shell__Shell__cmd_clear(%Shell* %self) {
shell__Shell__cmd_clear$main:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	call void @console__Console__clear(%Console* %0)
	ret void
}

define void @shell__Shell__cmd_about(%Shell* %self) {
shell__Shell__cmd_about$main:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%1 = load i8, i8* @LIGHT_CYAN
	%2 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %0, i8 %1, i8 %2)
	%3 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%4 = getelementptr [16 x i8], [16 x i8]* @.str.d19e1f2dc0f9f01aba007f4665999260, i8 0
	%5 = bitcast [16 x i8]* %4 to i8*
	call void @console__Console__print_str(%Console* %3, i8* %5)
	%6 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%7 = load i8, i8* @LIGHT_GRAY
	%8 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %6, i8 %7, i8 %8)
	%9 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%10 = getelementptr [46 x i8], [46 x i8]* @.str.8987c9f6edab3347f7705e66f961158e, i8 0
	%11 = bitcast [46 x i8]* %10 to i8*
	call void @console__Console__print_str(%Console* %9, i8* %11)
	%12 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%13 = getelementptr [11 x i8], [11 x i8]* @.str.59d4e94a1653ab2861ef47e3caeb580a, i8 0
	%14 = bitcast [11 x i8]* %13 to i8*
	call void @console__Console__print_str(%Console* %12, i8* %14)
	%15 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%16 = getelementptr [42 x i8], [42 x i8]* @.str.db4050860aa28162356f27aa713423cc, i8 0
	%17 = bitcast [42 x i8]* %16 to i8*
	call void @console__Console__print_str(%Console* %15, i8* %17)
	%18 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%19 = getelementptr [35 x i8], [35 x i8]* @.str.07e46d667a50bcacee8f890f1c573d52, i8 0
	%20 = bitcast [35 x i8]* %19 to i8*
	call void @console__Console__print_str(%Console* %18, i8* %20)
	%21 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%22 = getelementptr [27 x i8], [27 x i8]* @.str.bde5c99d7bb57c1815245963f27df98c, i8 0
	%23 = bitcast [27 x i8]* %22 to i8*
	call void @console__Console__print_str(%Console* %21, i8* %23)
	%24 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%25 = getelementptr [13 x i8], [13 x i8]* @.str.2a95262bdf56446a55105b4f08635b7c, i8 0
	%26 = bitcast [13 x i8]* %25 to i8*
	call void @console__Console__print_str(%Console* %24, i8* %26)
	%27 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%28 = getelementptr [32 x i8], [32 x i8]* @.str.75b2256a66d831e688f10ff8b2321da0, i8 0
	%29 = bitcast [32 x i8]* %28 to i8*
	call void @console__Console__print_str(%Console* %27, i8* %29)
	%30 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%31 = getelementptr [33 x i8], [33 x i8]* @.str.531ed4c67e1ae4bd108543997eb0f3d8, i8 0
	%32 = bitcast [33 x i8]* %31 to i8*
	call void @console__Console__print_str(%Console* %30, i8* %32)
	%33 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%34 = getelementptr [21 x i8], [21 x i8]* @.str.2771e045e897b27450254a8c3af506ba, i8 0
	%35 = bitcast [21 x i8]* %34 to i8*
	call void @console__Console__print_str(%Console* %33, i8* %35)
	ret void
}

define void @shell__Shell__cmd_echo(%Shell* %self) {
shell__Shell__cmd_echo$main:
	%i = alloca i64
	store i64 5, i64* %i
	br label %loop.header.82

loop.header.82:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 2
	%1 = load i64, i64* %0
	%2 = load i64, i64* %i
	%3 = icmp slt i64 %2, %1
	br i1 %3, label %loop.body.82, label %loop.exit.82

loop.body.82:
	%4 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%5 = getelementptr %Shell, %Shell* %self, i32 0, i32 1
	%6 = load i64, i64* %i
	%7 = getelementptr [256 x i8], [256 x i8]* %5, i64 0, i64 %6
	%8 = load i8, i8* %7
	call void @console__Console__putchar(%Console* %4, i8 %8)
	%9 = load i64, i64* %i
	%10 = add i64 %9, 1
	store i64 %10, i64* %i
	br label %loop.header.82

loop.exit.82:
	%11 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	call void @console__Console__putchar(%Console* %11, i8 10)
	ret void
}

define void @shell__Shell__cmd_color(%Shell* %self) {
shell__Shell__cmd_color$main:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%1 = load i8, i8* @LIGHT_GREEN
	%2 = load i8, i8* @BLACK
	call void @console__Console__set_color(%Console* %0, i8 %1, i8 %2)
	%3 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%4 = getelementptr [21 x i8], [21 x i8]* @.str.a5ef6a3809d4a7744e1a3cd3955d64bb, i8 0
	%5 = bitcast [21 x i8]* %4 to i8*
	call void @console__Console__print_str(%Console* %3, i8* %5)
	ret void
}

define void @shell__Shell__cmd_reboot(%Shell* %self) {
shell__Shell__cmd_reboot$main:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%1 = getelementptr [14 x i8], [14 x i8]* @.str.7a2cdb4d39f68772427d175ed39f6c95, i8 0
	%2 = bitcast [14 x i8]* %1 to i8*
	call void @console__Console__print_str(%Console* %0, i8* %2)
	call void asm sideeffect "movb $$0xFE, %al", ""()
	call void asm sideeffect "outb %al, $$0x64", ""()
	ret void
}

define void @shell__Shell__cmd_halt(%Shell* %self) {
shell__Shell__cmd_halt$main:
	%0 = getelementptr %Shell, %Shell* %self, i32 0, i32 0
	%1 = getelementptr [23 x i8], [23 x i8]* @.str.0319bb33ef05673833765b1bc4214e62, i8 0
	%2 = bitcast [23 x i8]* %1 to i8*
	call void @console__Console__print_str(%Console* %0, i8* %2)
	%3 = getelementptr %Shell, %Shell* %self, i32 0, i32 3
	store i1 false, i1* %3
	call void asm sideeffect "cli", ""()
	call void asm sideeffect "hlt", ""()
	ret void
}
