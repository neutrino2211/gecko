/* Multiboot header and entry point for Gecko kernel */

.set MAGIC,    0x1BADB002       /* Multiboot magic number */
.set FLAGS,    0x00000000       /* Multiboot flags */
.set CHECKSUM, -(MAGIC + FLAGS) /* Checksum (magic + flags + checksum = 0) */

/* Multiboot header - must be in first 8KB of kernel */
.section .multiboot, "a"
.align 4
multiboot_header:
    .long MAGIC
    .long FLAGS
    .long CHECKSUM

/* Stack - 16KB */
.section .bss
.align 16
stack_bottom:
    .skip 16384
stack_top:

/* Entry point */
.section .text.boot
.global _start
.type _start, @function
_start:
    /* Set up stack */
    mov $stack_top, %esp

    /* Clear EFLAGS */
    pushl $0
    popf

    /* Call kernel main */
    call kmain

    /* If kmain returns, hang */
    cli
.hang:
    hlt
    jmp .hang
