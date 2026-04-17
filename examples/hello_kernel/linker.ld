/* Linker script for x86 kernel */

/* Entry point */
ENTRY(_start)

/* Kernel loads at 1MB (standard for x86 kernels) */
KERNEL_BASE = 0x100000;

SECTIONS
{
    /* Start at 1MB */
    . = KERNEL_BASE;

    /* Multiboot header must be in first 8KB */
    .multiboot ALIGN(4) :
    {
        *(.multiboot)
    }

    /* Boot code section */
    .text.boot ALIGN(4096) :
    {
        *(.text.boot)
    }

    /* Main code section */
    .text ALIGN(4096) :
    {
        *(.text)
        *(.text.*)
    }

    /* Read-only data */
    .rodata ALIGN(4096) :
    {
        *(.rodata)
        *(.rodata.*)
    }

    /* Initialized data */
    .data ALIGN(4096) :
    {
        *(.data)
        *(.data.*)
    }

    /* Uninitialized data (BSS) */
    .bss ALIGN(4096) :
    {
        __bss_start = .;
        *(.bss)
        *(.bss.*)
        *(COMMON)
        __bss_end = .;
    }

    /* Stack (16KB, grows downward) */
    . = ALIGN(16);
    __stack_bottom = .;
    . += 16384;
    __stack_top = .;

    /* End of kernel */
    __kernel_end = .;

    /* Discard unwanted sections */
    /DISCARD/ :
    {
        *(.comment)
        *(.note.*)
        *(.eh_frame*)
    }
}
