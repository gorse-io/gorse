//go:build !noasm && arm64
// AUTO-GENERATED BY GOAT -- DO NOT EDIT

TEXT ·_scan(SB), $0-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD n+16(FP), R2
	MOVD ret+24(FP), R3
	MOVD n_ret+32(FP), R4
	WORD $0xa9bf7bfd      // stp	x29, x30, [sp,
	WORD $0x3dc00000      // ldr	q0, [x0]
	WORD $0x4e010c21      // dup	v1.16b, w1
	WORD $0x910003fd      // mov	x29, sp
	WORD $0xf900009f      // str	xzr, [x4]
	WORD $0x6e218c00      // cmeq	v0.16b, v0.16b, v1.16b
	WORD $0x0f0c8400      // shrn	v0.8b, v0.8h,
	WORD $0x9e660008      // fmov	x8, d0
	WORD $0xf201e108      // ands	x8, x8,
	WORD $0x540001c0      // b.eq	.LBB0_4
	WORD $0xaa1f03e9      // mov	x9, xzr

LBB0_2:
	WORD $0xdac0010a // rbit	x10, x8
	WORD $0xdac0114a // clz	x10, x10
	WORD $0xd342fd4a // lsr	x10, x10,
	WORD $0xeb02015f // cmp	x10, x2
	WORD $0x5400010a // b.ge	.LBB0_4
	WORD $0xf829786a // str	x10, [x3, x9, lsl
	WORD $0xf9400089 // ldr	x9, [x4]
	WORD $0xd100050a // sub	x10, x8,
	WORD $0xea080148 // ands	x8, x10, x8
	WORD $0x91000529 // add	x9, x9,
	WORD $0xf9000089 // str	x9, [x4]
	WORD $0x54fffea1 // b.ne	.LBB0_2

LBB0_4:
	WORD $0xa8c17bfd // ldp	x29, x30, [sp],
	WORD $0xd65f03c0 // ret