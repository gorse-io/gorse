//+build !noasm !appengine
// AUTO-GENERATED -- DO NOT EDIT

TEXT ·vmul_const_add_to(SB), $0-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD c+16(FP), R2
	MOVD n+24(FP), R3
	WORD $0xa9bf7bfd // stp	x29, x30, [sp, #-16]!
	WORD $0x91000c68 // add	x8, x3, #3
	WORD $0xf100007f // cmp	x3, #0
	WORD $0x9a83b108 // csel	x8, x8, x3, lt
	WORD $0xd342fd0a // lsr	x10, x8, #2
	WORD $0x927ef508 // and	x8, x8, #0xfffffffffffffffc
	WORD $0x7100055f // cmp	w10, #1
	WORD $0xcb080069 // sub	x9, x3, x8
	WORD $0x910003fd // mov	x29, sp
	WORD $0x540001cb // b.lt	.LBB0_4
	WORD $0xaa0203e8 // mov	x8, x2
LBB0_2:
	WORD $0x3cc10400 // ldr	q0, [x0], #16
	WORD $0x3cc10501 // ldr	q1, [x8], #16
	WORD $0xbd400022 // ldr	s2, [x1]
	WORD $0x7100054a // subs	w10, w10, #1
	WORD $0x4f829000 // fmul	v0.4s, v0.4s, v2.s[0]
	WORD $0x4e20d420 // fadd	v0.4s, v1.4s, v0.4s
	WORD $0x3d800040 // str	q0, [x2]
	WORD $0xaa0803e2 // mov	x2, x8
	WORD $0x54ffff01 // b.ne	.LBB0_2
	WORD $0x7100053f // cmp	w9, #1
	WORD $0x540000aa // b.ge	.LBB0_5
	WORD $0x1400000d // b	.LBB0_7
	WORD $0xaa0203e8 // mov	x8, x2
	WORD $0x7100053f // cmp	w9, #1
	WORD $0x5400014b // b.lt	.LBB0_7
	WORD $0x92407d29 // and	x9, x9, #0xffffffff
LBB0_6:
	WORD $0xbc404400 // ldr	s0, [x0], #4
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0xbd400102 // ldr	s2, [x8]
	WORD $0xf1000529 // subs	x9, x9, #1
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0x1e202840 // fadd	s0, s2, s0
	WORD $0xbc004500 // str	s0, [x8], #4
	WORD $0x54ffff21 // b.ne	.LBB0_6
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET

TEXT ·vmul_const_to(SB), $0-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD c+16(FP), R2
	MOVD n+24(FP), R3
	WORD $0xa9bf7bfd // stp	x29, x30, [sp, #-16]!
	WORD $0x91000c68 // add	x8, x3, #3
	WORD $0xf100007f // cmp	x3, #0
	WORD $0x9a83b108 // csel	x8, x8, x3, lt
	WORD $0xd342fd09 // lsr	x9, x8, #2
	WORD $0x927ef508 // and	x8, x8, #0xfffffffffffffffc
	WORD $0x7100053f // cmp	w9, #1
	WORD $0xcb080068 // sub	x8, x3, x8
	WORD $0x910003fd // mov	x29, sp
	WORD $0x540000eb // b.lt	.LBB1_2
LBB1_1:
	WORD $0x3cc10400 // ldr	q0, [x0], #16
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0x71000529 // subs	w9, w9, #1
	WORD $0x4f819000 // fmul	v0.4s, v0.4s, v1.s[0]
	WORD $0x3c810440 // str	q0, [x2], #16
	WORD $0x54ffff61 // b.ne	.LBB1_1
	WORD $0x7100051f // cmp	w8, #1
	WORD $0x5400010b // b.lt	.LBB1_5
	WORD $0x92407d08 // and	x8, x8, #0xffffffff
LBB1_4:
	WORD $0xbc404400 // ldr	s0, [x0], #4
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0xf1000508 // subs	x8, x8, #1
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0xbc004440 // str	s0, [x2], #4
	WORD $0x54ffff61 // b.ne	.LBB1_4
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET

TEXT ·vmul_const(SB), $0-24
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD n+16(FP), R2
	WORD $0xa9bf7bfd // stp	x29, x30, [sp, #-16]!
	WORD $0x91000c48 // add	x8, x2, #3
	WORD $0xf100005f // cmp	x2, #0
	WORD $0x9a82b108 // csel	x8, x8, x2, lt
	WORD $0xd342fd0a // lsr	x10, x8, #2
	WORD $0x927ef508 // and	x8, x8, #0xfffffffffffffffc
	WORD $0x7100055f // cmp	w10, #1
	WORD $0xcb080049 // sub	x9, x2, x8
	WORD $0x910003fd // mov	x29, sp
	WORD $0x5400018b // b.lt	.LBB2_4
	WORD $0xaa0003e8 // mov	x8, x0
LBB2_2:
	WORD $0x3cc10500 // ldr	q0, [x8], #16
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0x7100054a // subs	w10, w10, #1
	WORD $0x4f819000 // fmul	v0.4s, v0.4s, v1.s[0]
	WORD $0x3d800000 // str	q0, [x0]
	WORD $0xaa0803e0 // mov	x0, x8
	WORD $0x54ffff41 // b.ne	.LBB2_2
	WORD $0x7100053f // cmp	w9, #1
	WORD $0x540000aa // b.ge	.LBB2_5
	WORD $0x1400000b // b	.LBB2_7
	WORD $0xaa0003e8 // mov	x8, x0
	WORD $0x7100053f // cmp	w9, #1
	WORD $0x5400010b // b.lt	.LBB2_7
	WORD $0x92407d29 // and	x9, x9, #0xffffffff
LBB2_6:
	WORD $0xbd400020 // ldr	s0, [x1]
	WORD $0xbd400101 // ldr	s1, [x8]
	WORD $0xf1000529 // subs	x9, x9, #1
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0xbc004500 // str	s0, [x8], #4
	WORD $0x54ffff61 // b.ne	.LBB2_6
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET

TEXT ·vmul_to(SB), $0-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD c+16(FP), R2
	MOVD n+24(FP), R3
	WORD $0xa9bf7bfd // stp	x29, x30, [sp, #-16]!
	WORD $0x91000c68 // add	x8, x3, #3
	WORD $0xf100007f // cmp	x3, #0
	WORD $0x9a83b108 // csel	x8, x8, x3, lt
	WORD $0xd342fd09 // lsr	x9, x8, #2
	WORD $0x927ef508 // and	x8, x8, #0xfffffffffffffffc
	WORD $0x7100053f // cmp	w9, #1
	WORD $0xcb080068 // sub	x8, x3, x8
	WORD $0x910003fd // mov	x29, sp
	WORD $0x540000eb // b.lt	.LBB3_2
LBB3_1:
	WORD $0x3cc10400 // ldr	q0, [x0], #16
	WORD $0x3cc10421 // ldr	q1, [x1], #16
	WORD $0x71000529 // subs	w9, w9, #1
	WORD $0x6e21dc00 // fmul	v0.4s, v0.4s, v1.4s
	WORD $0x3c810440 // str	q0, [x2], #16
	WORD $0x54ffff61 // b.ne	.LBB3_1
	WORD $0x7100051f // cmp	w8, #1
	WORD $0x5400010b // b.lt	.LBB3_5
	WORD $0x92407d08 // and	x8, x8, #0xffffffff
LBB3_4:
	WORD $0xbc404400 // ldr	s0, [x0], #4
	WORD $0xbc404421 // ldr	s1, [x1], #4
	WORD $0xf1000508 // subs	x8, x8, #1
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0xbc004440 // str	s0, [x2], #4
	WORD $0x54ffff61 // b.ne	.LBB3_4
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET

TEXT ·vdot(SB), $0-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD n+16(FP), R2
	MOVD ret+24(FP), R3
	WORD $0xa9bf7bfd // stp	x29, x30, [sp, #-16]!
	WORD $0xd10043e9 // sub	x9, sp, #16
	WORD $0x910003fd // mov	x29, sp
	WORD $0x927ced3f // and	sp, x9, #0xfffffffffffffff0
	WORD $0x91000c48 // add	x8, x2, #3
	WORD $0xf100005f // cmp	x2, #0
	WORD $0x9a82b109 // csel	x9, x8, x2, lt
	WORD $0x9342fd28 // asr	x8, x9, #2
	WORD $0x7100051f // cmp	w8, #1
	WORD $0x927ef529 // and	x9, x9, #0xfffffffffffffffc
	WORD $0x540000ab // b.lt	.LBB4_2
	WORD $0x3cc10400 // ldr	q0, [x0], #16
	WORD $0x3cc10421 // ldr	q1, [x1], #16
	WORD $0x6e21dc00 // fmul	v0.4s, v0.4s, v1.4s
	WORD $0x14000001 // b	.LBB4_3
	WORD $0x7100050a // subs	w10, w8, #1
	WORD $0xcb090049 // sub	x9, x2, x9
	WORD $0x540001ed // b.le	.LBB4_7
	WORD $0xb27d7beb // mov	x11, #17179869176
	WORD $0x8b080968 // add	x8, x11, x8, lsl #2
	WORD $0x927e7d08 // and	x8, x8, #0x3fffffffc
	WORD $0x9100110b // add	x11, x8, #4
	WORD $0x8b0b0808 // add	x8, x0, x11, lsl #2
	WORD $0xaa0103ec // mov	x12, x1
LBB4_5:
	WORD $0x3cc10401 // ldr	q1, [x0], #16
	WORD $0x3cc10582 // ldr	q2, [x12], #16
	WORD $0x7100054a // subs	w10, w10, #1
	WORD $0x6e22dc21 // fmul	v1.4s, v1.4s, v2.4s
	WORD $0x4e21d400 // fadd	v0.4s, v0.4s, v1.4s
	WORD $0x54ffff61 // b.ne	.LBB4_5
	WORD $0x8b0b0821 // add	x1, x1, x11, lsl #2
	WORD $0x14000002 // b	.LBB4_8
	WORD $0xaa0003e8 // mov	x8, x0
	WORD $0x3d8003e0 // str	q0, [sp]
	WORD $0xbd400060 // ldr	s0, [x3]
	WORD $0xaa1f03ea // mov	x10, xzr
	WORD $0x910003eb // mov	x11, sp
LBB4_9:
	WORD $0xbc6a6961 // ldr	s1, [x11, x10]
	WORD $0x9100114a // add	x10, x10, #4
	WORD $0xf100415f // cmp	x10, #16
	WORD $0x1e202820 // fadd	s0, s1, s0
	WORD $0x54ffff81 // b.ne	.LBB4_9
	WORD $0x7100053f // cmp	w9, #1
	WORD $0xbd000060 // str	s0, [x3]
	WORD $0x5400014b // b.lt	.LBB4_13
	WORD $0x92407d29 // and	x9, x9, #0xffffffff
LBB4_12:
	WORD $0xbc404500 // ldr	s0, [x8], #4
	WORD $0xbc404421 // ldr	s1, [x1], #4
	WORD $0xbd400062 // ldr	s2, [x3]
	WORD $0xf1000529 // subs	x9, x9, #1
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0x1e202840 // fadd	s0, s2, s0
	WORD $0xbd000060 // str	s0, [x3]
	WORD $0x54ffff21 // b.ne	.LBB4_12
	WORD $0x910003bf // mov	sp, x29
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET
