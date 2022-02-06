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
	WORD $0xd342fd09 // lsr	x9, x8, #2
	WORD $0x927ef508 // and	x8, x8, #0xfffffffffffffffc
	WORD $0x7100053f // cmp	w9, #1
	WORD $0xcb08006b // sub	x11, x3, x8
	WORD $0x910003fd // mov	x29, sp
	WORD $0x540001cb // b.lt	.LBB0_4
	WORD $0xaa0203e8 // mov	x8, x2
LBB0_2:
	WORD $0x3cc10400 // ldr	q0, [x0], #16
	WORD $0x3cc10501 // ldr	q1, [x8], #16
	WORD $0xbd400022 // ldr	s2, [x1]
	WORD $0x71000529 // subs	w9, w9, #1
	WORD $0x4f829000 // fmul	v0.4s, v0.4s, v2.s[0]
	WORD $0x4e20d420 // fadd	v0.4s, v1.4s, v0.4s
	WORD $0x3d800040 // str	q0, [x2]
	WORD $0xaa0803e2 // mov	x2, x8
	WORD $0x54ffff01 // b.ne	.LBB0_2
	WORD $0x7100057f // cmp	w11, #1
	WORD $0x540000aa // b.ge	.LBB0_5
	WORD $0x14000038 // b	.LBB0_14
	WORD $0xaa0203e8 // mov	x8, x2
	WORD $0x7100057f // cmp	w11, #1
	WORD $0x540006ab // b.lt	.LBB0_14
	WORD $0x92407d69 // and	x9, x11, #0xffffffff
	WORD $0xf1001d3f // cmp	x9, #7
	WORD $0x54000068 // b.hi	.LBB0_7
	WORD $0xaa1f03ea // mov	x10, xzr
	WORD $0x14000024 // b	.LBB0_12
	WORD $0xd37ef52c // lsl	x12, x9, #2
	WORD $0x8b0c010e // add	x14, x8, x12
	WORD $0x8b0c000c // add	x12, x0, x12
	WORD $0xeb0c011f // cmp	x8, x12
	WORD $0x9100042d // add	x13, x1, #1
	WORD $0x1a9f27ec // cset	w12, lo
	WORD $0xeb0e001f // cmp	x0, x14
	WORD $0x1a9f27ef // cset	w15, lo
	WORD $0xeb0801bf // cmp	x13, x8
	WORD $0x0a0f018f // and	w15, w12, w15
	WORD $0x1a9f97ec // cset	w12, hi
	WORD $0xeb0101df // cmp	x14, x1
	WORD $0xaa1f03ea // mov	x10, xzr
	WORD $0x1a9f97ed // cset	w13, hi
	WORD $0x370002af // tbnz	w15, #0, .LBB0_12
	WORD $0x0a0d018c // and	w12, w12, w13
	WORD $0x3700026c // tbnz	w12, #0, .LBB0_12
	WORD $0x4d40c820 // ld1r	{ v0.4s }, [x1]
	WORD $0x9240096b // and	x11, x11, #0x7
	WORD $0xcb0b012a // sub	x10, x9, x11
	WORD $0x9100400c // add	x12, x0, #16
	WORD $0x9100410d // add	x13, x8, #16
	WORD $0xaa0a03ee // mov	x14, x10
LBB0_10:
	WORD $0xad7f8981 // ldp	q1, q2, [x12, #-16]
	WORD $0xad7f91a3 // ldp	q3, q4, [x13, #-16]
	WORD $0x9100818c // add	x12, x12, #32
	WORD $0xf10021ce // subs	x14, x14, #8
	WORD $0x6e20dc21 // fmul	v1.4s, v1.4s, v0.4s
	WORD $0x6e20dc42 // fmul	v2.4s, v2.4s, v0.4s
	WORD $0x4e21d461 // fadd	v1.4s, v3.4s, v1.4s
	WORD $0x4e22d482 // fadd	v2.4s, v4.4s, v2.4s
	WORD $0xad3f89a1 // stp	q1, q2, [x13, #-16]
	WORD $0x910081ad // add	x13, x13, #32
	WORD $0x54fffec1 // b.ne	.LBB0_10
	WORD $0xb40001ab // cbz	x11, .LBB0_14
	WORD $0xd37ef54b // lsl	x11, x10, #2
	WORD $0x8b0b0108 // add	x8, x8, x11
	WORD $0x8b0b000b // add	x11, x0, x11
	WORD $0xcb0a0129 // sub	x9, x9, x10
LBB0_13:
	WORD $0xbc404560 // ldr	s0, [x11], #4
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0xbd400102 // ldr	s2, [x8]
	WORD $0xf1000529 // subs	x9, x9, #1
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0x1e202840 // fadd	s0, s2, s0
	WORD $0xbc004500 // str	s0, [x8], #4
	WORD $0x54ffff21 // b.ne	.LBB0_13
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
	WORD $0x9a83b109 // csel	x9, x8, x3, lt
	WORD $0xd342fd28 // lsr	x8, x9, #2
	WORD $0x927ef529 // and	x9, x9, #0xfffffffffffffffc
	WORD $0x7100051f // cmp	w8, #1
	WORD $0xcb090069 // sub	x9, x3, x9
	WORD $0x910003fd // mov	x29, sp
	WORD $0x540000eb // b.lt	.LBB1_2
LBB1_1:
	WORD $0x3cc10400 // ldr	q0, [x0], #16
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0x71000508 // subs	w8, w8, #1
	WORD $0x4f819000 // fmul	v0.4s, v0.4s, v1.s[0]
	WORD $0x3c810440 // str	q0, [x2], #16
	WORD $0x54ffff61 // b.ne	.LBB1_1
	WORD $0x7100053f // cmp	w9, #1
	WORD $0x5400060b // b.lt	.LBB1_12
	WORD $0x92407d28 // and	x8, x9, #0xffffffff
	WORD $0xf1001d1f // cmp	x8, #7
	WORD $0x54000068 // b.hi	.LBB1_5
	WORD $0xaa1f03ea // mov	x10, xzr
	WORD $0x14000021 // b	.LBB1_10
	WORD $0xd37ef50b // lsl	x11, x8, #2
	WORD $0x8b0b004d // add	x13, x2, x11
	WORD $0x8b0b000b // add	x11, x0, x11
	WORD $0xeb0b005f // cmp	x2, x11
	WORD $0x9100042c // add	x12, x1, #1
	WORD $0x1a9f27eb // cset	w11, lo
	WORD $0xeb0d001f // cmp	x0, x13
	WORD $0x1a9f27ee // cset	w14, lo
	WORD $0xeb02019f // cmp	x12, x2
	WORD $0x0a0e016e // and	w14, w11, w14
	WORD $0x1a9f97eb // cset	w11, hi
	WORD $0xeb0101bf // cmp	x13, x1
	WORD $0xaa1f03ea // mov	x10, xzr
	WORD $0x1a9f97ec // cset	w12, hi
	WORD $0x3700024e // tbnz	w14, #0, .LBB1_10
	WORD $0x0a0c016b // and	w11, w11, w12
	WORD $0x3700020b // tbnz	w11, #0, .LBB1_10
	WORD $0x4d40c820 // ld1r	{ v0.4s }, [x1]
	WORD $0x92400929 // and	x9, x9, #0x7
	WORD $0xcb09010a // sub	x10, x8, x9
	WORD $0x9100400b // add	x11, x0, #16
	WORD $0x9100404c // add	x12, x2, #16
	WORD $0xaa0a03ed // mov	x13, x10
LBB1_8:
	WORD $0xad7f8961 // ldp	q1, q2, [x11, #-16]
	WORD $0x9100816b // add	x11, x11, #32
	WORD $0xf10021ad // subs	x13, x13, #8
	WORD $0x6e20dc21 // fmul	v1.4s, v1.4s, v0.4s
	WORD $0x6e20dc42 // fmul	v2.4s, v2.4s, v0.4s
	WORD $0xad3f8981 // stp	q1, q2, [x12, #-16]
	WORD $0x9100818c // add	x12, x12, #32
	WORD $0x54ffff21 // b.ne	.LBB1_8
	WORD $0xb4000169 // cbz	x9, .LBB1_12
	WORD $0xd37ef54b // lsl	x11, x10, #2
	WORD $0x8b0b0049 // add	x9, x2, x11
	WORD $0x8b0b000b // add	x11, x0, x11
	WORD $0xcb0a0108 // sub	x8, x8, x10
LBB1_11:
	WORD $0xbc404560 // ldr	s0, [x11], #4
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0xf1000508 // subs	x8, x8, #1
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0xbc004520 // str	s0, [x9], #4
	WORD $0x54ffff61 // b.ne	.LBB1_11
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
	WORD $0xd342fd09 // lsr	x9, x8, #2
	WORD $0x927ef508 // and	x8, x8, #0xfffffffffffffffc
	WORD $0x7100053f // cmp	w9, #1
	WORD $0xcb08004a // sub	x10, x2, x8
	WORD $0x910003fd // mov	x29, sp
	WORD $0x5400018b // b.lt	.LBB2_4
	WORD $0xaa0003e8 // mov	x8, x0
LBB2_2:
	WORD $0x3cc10500 // ldr	q0, [x8], #16
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0x71000529 // subs	w9, w9, #1
	WORD $0x4f819000 // fmul	v0.4s, v0.4s, v1.s[0]
	WORD $0x3d800000 // str	q0, [x0]
	WORD $0xaa0803e0 // mov	x0, x8
	WORD $0x54ffff41 // b.ne	.LBB2_2
	WORD $0x7100055f // cmp	w10, #1
	WORD $0x540000aa // b.ge	.LBB2_5
	WORD $0x14000016 // b	.LBB2_11
	WORD $0xaa0003e8 // mov	x8, x0
	WORD $0x7100055f // cmp	w10, #1
	WORD $0x5400026b // b.lt	.LBB2_11
	WORD $0x92407d49 // and	x9, x10, #0xffffffff
	WORD $0xf1001d3f // cmp	x9, #7
	WORD $0x540000e9 // b.ls	.LBB2_8
	WORD $0x9100042b // add	x11, x1, #1
	WORD $0xeb08017f // cmp	x11, x8
	WORD $0x540001e9 // b.ls	.LBB2_12
	WORD $0x8b09090b // add	x11, x8, x9, lsl #2
	WORD $0xeb01017f // cmp	x11, x1
	WORD $0x54000189 // b.ls	.LBB2_12
	WORD $0xaa1f03ea // mov	x10, xzr
	WORD $0x8b0a0908 // add	x8, x8, x10, lsl #2
	WORD $0xcb0a0129 // sub	x9, x9, x10
LBB2_10:
	WORD $0xbd400020 // ldr	s0, [x1]
	WORD $0xbd400101 // ldr	s1, [x8]
	WORD $0xf1000529 // subs	x9, x9, #1
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0xbc004500 // str	s0, [x8], #4
	WORD $0x54ffff61 // b.ne	.LBB2_10
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET
	WORD $0x4d40c820 // ld1r	{ v0.4s }, [x1]
	WORD $0x9240094b // and	x11, x10, #0x7
	WORD $0xcb0b012a // sub	x10, x9, x11
	WORD $0x9100410c // add	x12, x8, #16
	WORD $0xaa0a03ed // mov	x13, x10
LBB2_13:
	WORD $0xad7f8981 // ldp	q1, q2, [x12, #-16]
	WORD $0xf10021ad // subs	x13, x13, #8
	WORD $0x6e21dc01 // fmul	v1.4s, v0.4s, v1.4s
	WORD $0x6e22dc02 // fmul	v2.4s, v0.4s, v2.4s
	WORD $0xad3f8981 // stp	q1, q2, [x12, #-16]
	WORD $0x9100818c // add	x12, x12, #32
	WORD $0x54ffff41 // b.ne	.LBB2_13
	WORD $0xb5fffd4b // cbnz	x11, .LBB2_9
	WORD $0x17fffff1 // b	.LBB2_11

TEXT ·vmul_to(SB), $0-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD c+16(FP), R2
	MOVD n+24(FP), R3
	WORD $0xa9bf7bfd // stp	x29, x30, [sp, #-16]!
	WORD $0x91000c68 // add	x8, x3, #3
	WORD $0xf100007f // cmp	x3, #0
	WORD $0x9a83b109 // csel	x9, x8, x3, lt
	WORD $0xd342fd28 // lsr	x8, x9, #2
	WORD $0x927ef529 // and	x9, x9, #0xfffffffffffffffc
	WORD $0x7100051f // cmp	w8, #1
	WORD $0xcb09006a // sub	x10, x3, x9
	WORD $0x910003fd // mov	x29, sp
	WORD $0x540000eb // b.lt	.LBB3_2
LBB3_1:
	WORD $0x3cc10400 // ldr	q0, [x0], #16
	WORD $0x3cc10421 // ldr	q1, [x1], #16
	WORD $0x71000508 // subs	w8, w8, #1
	WORD $0x6e21dc00 // fmul	v0.4s, v0.4s, v1.4s
	WORD $0x3c810440 // str	q0, [x2], #16
	WORD $0x54ffff61 // b.ne	.LBB3_1
	WORD $0x7100055f // cmp	w10, #1
	WORD $0x5400066b // b.lt	.LBB3_12
	WORD $0x92407d48 // and	x8, x10, #0xffffffff
	WORD $0xf1001d1f // cmp	x8, #7
	WORD $0x54000068 // b.hi	.LBB3_5
	WORD $0xaa1f03e9 // mov	x9, xzr
	WORD $0x14000023 // b	.LBB3_10
	WORD $0xd37ef50b // lsl	x11, x8, #2
	WORD $0x8b0b000d // add	x13, x0, x11
	WORD $0x8b0b004c // add	x12, x2, x11
	WORD $0xeb0d005f // cmp	x2, x13
	WORD $0x8b0b002b // add	x11, x1, x11
	WORD $0x1a9f27ed // cset	w13, lo
	WORD $0xeb0c001f // cmp	x0, x12
	WORD $0x1a9f27ee // cset	w14, lo
	WORD $0xeb0b005f // cmp	x2, x11
	WORD $0x1a9f27eb // cset	w11, lo
	WORD $0xeb0c003f // cmp	x1, x12
	WORD $0xaa1f03e9 // mov	x9, xzr
	WORD $0x0a0e01ad // and	w13, w13, w14
	WORD $0x1a9f27ec // cset	w12, lo
	WORD $0x3700028d // tbnz	w13, #0, .LBB3_10
	WORD $0x0a0c016b // and	w11, w11, w12
	WORD $0x3700024b // tbnz	w11, #0, .LBB3_10
	WORD $0x9240094a // and	x10, x10, #0x7
	WORD $0xcb0a0109 // sub	x9, x8, x10
	WORD $0x9100400b // add	x11, x0, #16
	WORD $0x9100402c // add	x12, x1, #16
	WORD $0x9100404d // add	x13, x2, #16
	WORD $0xaa0903ee // mov	x14, x9
LBB3_8:
	WORD $0xad7f8560 // ldp	q0, q1, [x11, #-16]
	WORD $0xad7f8d82 // ldp	q2, q3, [x12, #-16]
	WORD $0x9100816b // add	x11, x11, #32
	WORD $0x9100818c // add	x12, x12, #32
	WORD $0xf10021ce // subs	x14, x14, #8
	WORD $0x6e22dc00 // fmul	v0.4s, v0.4s, v2.4s
	WORD $0x6e23dc21 // fmul	v1.4s, v1.4s, v3.4s
	WORD $0xad3f85a0 // stp	q0, q1, [x13, #-16]
	WORD $0x910081ad // add	x13, x13, #32
	WORD $0x54fffee1 // b.ne	.LBB3_8
	WORD $0xb400018a // cbz	x10, .LBB3_12
	WORD $0xd37ef52c // lsl	x12, x9, #2
	WORD $0x8b0c004a // add	x10, x2, x12
	WORD $0x8b0c002b // add	x11, x1, x12
	WORD $0x8b0c000c // add	x12, x0, x12
	WORD $0xcb090108 // sub	x8, x8, x9
LBB3_11:
	WORD $0xbc404580 // ldr	s0, [x12], #4
	WORD $0xbc404561 // ldr	s1, [x11], #4
	WORD $0xf1000508 // subs	x8, x8, #1
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0xbc004540 // str	s0, [x10], #4
	WORD $0x54ffff61 // b.ne	.LBB3_11
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET

TEXT ·vdot(SB), $0-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD n+16(FP), R2
	MOVD ret+24(FP), R3
	WORD $0xa9bf7bfd // stp	x29, x30, [sp, #-16]!
	WORD $0x91000c48 // add	x8, x2, #3
	WORD $0xf100005f // cmp	x2, #0
	WORD $0x9a82b108 // csel	x8, x8, x2, lt
	WORD $0x9342fd0a // asr	x10, x8, #2
	WORD $0x927ef508 // and	x8, x8, #0xfffffffffffffffc
	WORD $0x7100055f // cmp	w10, #1
	WORD $0xcb080048 // sub	x8, x2, x8
	WORD $0x910003fd // mov	x29, sp
	WORD $0x540002ab // b.lt	.LBB4_5
	WORD $0x3cc10400 // ldr	q0, [x0], #16
	WORD $0x3cc10421 // ldr	q1, [x1], #16
	WORD $0x71000549 // subs	w9, w10, #1
	WORD $0x6e21dc00 // fmul	v0.4s, v0.4s, v1.4s
	WORD $0x54000200 // b.eq	.LBB4_6
	WORD $0xb27d7beb // mov	x11, #17179869176
	WORD $0x8b0a096a // add	x10, x11, x10, lsl #2
	WORD $0x927e7d4a // and	x10, x10, #0x3fffffffc
	WORD $0x9100114b // add	x11, x10, #4
	WORD $0x8b0b080a // add	x10, x0, x11, lsl #2
	WORD $0xaa0103ec // mov	x12, x1
LBB4_3:
	WORD $0x3cc10401 // ldr	q1, [x0], #16
	WORD $0x3cc10582 // ldr	q2, [x12], #16
	WORD $0x71000529 // subs	w9, w9, #1
	WORD $0x6e22dc21 // fmul	v1.4s, v1.4s, v2.4s
	WORD $0x4e21d400 // fadd	v0.4s, v0.4s, v1.4s
	WORD $0x54ffff61 // b.ne	.LBB4_3
	WORD $0x8b0b0821 // add	x1, x1, x11, lsl #2
	WORD $0xaa0a03e0 // mov	x0, x10
	WORD $0x14000001 // b	.LBB4_6
	WORD $0xbd400061 // ldr	s1, [x3]
	WORD $0x5e0c0402 // mov	s2, v0.s[1]
	WORD $0x5e140403 // mov	s3, v0.s[2]
	WORD $0x5e1c0404 // mov	s4, v0.s[3]
	WORD $0x1e202820 // fadd	s0, s1, s0
	WORD $0x1e202840 // fadd	s0, s2, s0
	WORD $0x1e202860 // fadd	s0, s3, s0
	WORD $0x1e202880 // fadd	s0, s4, s0
	WORD $0x7100011f // cmp	w8, #0
	WORD $0xbd000060 // str	s0, [x3]
	WORD $0x5400012d // b.le	.LBB4_9
	WORD $0x92407d08 // and	x8, x8, #0xffffffff
LBB4_8:
	WORD $0xbc404401 // ldr	s1, [x0], #4
	WORD $0xbc404422 // ldr	s2, [x1], #4
	WORD $0xf1000508 // subs	x8, x8, #1
	WORD $0x1e220821 // fmul	s1, s1, s2
	WORD $0x1e212800 // fadd	s0, s0, s1
	WORD $0xbd000060 // str	s0, [x3]
	WORD $0x54ffff41 // b.ne	.LBB4_8
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET
