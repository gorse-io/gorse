//go:build !noasm && arm64
// Code generated by GoAT. DO NOT EDIT.

TEXT ·vmul_const_add_to(SB), $0-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD c+16(FP), R2
	MOVD n+24(FP), R3
	WORD $0xa9bf7bfd  // stp	x29, x30, [sp, #-16]!
	WORD $0x91000c68  // add	x8, x3, #3
	WORD $0xf100007f  // cmp	x3, #0
	WORD $0x910003fd  // mov	x29, sp
	WORD $0x9a83b109  // csel	x9, x8, x3, lt
	WORD $0xd342fd28  // lsr	x8, x9, #2
	WORD $0x927ef529  // and	x9, x9, #0xfffffffffffffffc
	WORD $0xcb090069  // sub	x9, x3, x9
	WORD $0x7100051f  // cmp	w8, #1
	WORD $0x5400010b  // b.lt	.LBB0_2

LBB0_1:
	WORD $0x3cc10400 // ldr	q0, [x0], #16
	WORD $0x3dc00041 // ldr	q1, [x2]
	WORD $0xbd400022 // ldr	s2, [x1]
	WORD $0x71000508 // subs	w8, w8, #1
	WORD $0x4f821001 // fmla	v1.4s, v0.4s, v2.s[0]
	WORD $0x3c810441 // str	q1, [x2], #16
	WORD $0x54ffff41 // b.ne	.LBB0_1

LBB0_2:
	WORD $0x7100053f // cmp	w9, #1
	WORD $0x540005ab // b.lt	.LBB0_12
	WORD $0x92407d28 // and	x8, x9, #0xffffffff
	WORD $0xf100211f // cmp	x8, #8
	WORD $0x54000062 // b.hs	.LBB0_5
	WORD $0xaa1f03ea // mov	x10, xzr
	WORD $0x1400001d // b	.LBB0_10

LBB0_5:
	WORD $0xd37ef50a // lsl	x10, x8, #2
	WORD $0x9100102b // add	x11, x1, #4
	WORD $0xeb0b005f // cmp	x2, x11
	WORD $0x8b0a004c // add	x12, x2, x10
	WORD $0x8b0a000a // add	x10, x0, x10
	WORD $0xfa413180 // ccmp	x12, x1, #0, lo
	WORD $0x1a9f97eb // cset	w11, hi
	WORD $0xeb0c001f // cmp	x0, x12
	WORD $0xfa4a3042 // ccmp	x2, x10, #2, lo
	WORD $0xaa1f03ea // mov	x10, xzr
	WORD $0x54000243 // b.lo	.LBB0_10
	WORD $0x3700022b // tbnz	w11, #0, .LBB0_10
	WORD $0x92400929 // and	x9, x9, #0x7
	WORD $0x4d40c820 // ld1r	{ v0.4s }, [x1]
	WORD $0x9100400b // add	x11, x0, #16
	WORD $0xcb09010a // sub	x10, x8, x9
	WORD $0x9100404c // add	x12, x2, #16
	WORD $0xaa0a03ed // mov	x13, x10

LBB0_8:
	WORD $0xad7f9181 // ldp	q1, q4, [x12, #-16]
	WORD $0xf10021ad // subs	x13, x13, #8
	WORD $0xad7f8d62 // ldp	q2, q3, [x11, #-16]
	WORD $0x9100816b // add	x11, x11, #32
	WORD $0x4e22cc01 // fmla	v1.4s, v0.4s, v2.4s
	WORD $0x4e23cc04 // fmla	v4.4s, v0.4s, v3.4s
	WORD $0xad3f9181 // stp	q1, q4, [x12, #-16]
	WORD $0x9100818c // add	x12, x12, #32
	WORD $0x54ffff01 // b.ne	.LBB0_8
	WORD $0xb4000189 // cbz	x9, .LBB0_12

LBB0_10:
	WORD $0xd37ef54b // lsl	x11, x10, #2
	WORD $0xcb0a0108 // sub	x8, x8, x10
	WORD $0x8b0b0049 // add	x9, x2, x11
	WORD $0x8b0b000b // add	x11, x0, x11

LBB0_11:
	WORD $0xbc404560 // ldr	s0, [x11], #4
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0xbd400122 // ldr	s2, [x9]
	WORD $0xf1000508 // subs	x8, x8, #1
	WORD $0x1f010800 // fmadd	s0, s0, s1, s2
	WORD $0xbc004520 // str	s0, [x9], #4
	WORD $0x54ffff41 // b.ne	.LBB0_11

LBB0_12:
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET

TEXT ·vmul_const_to(SB), $0-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD c+16(FP), R2
	MOVD n+24(FP), R3
	WORD $0xa9bf7bfd  // stp	x29, x30, [sp, #-16]!
	WORD $0x91000c68  // add	x8, x3, #3
	WORD $0xf100007f  // cmp	x3, #0
	WORD $0x910003fd  // mov	x29, sp
	WORD $0x9a83b109  // csel	x9, x8, x3, lt
	WORD $0xd342fd28  // lsr	x8, x9, #2
	WORD $0x927ef529  // and	x9, x9, #0xfffffffffffffffc
	WORD $0xcb090069  // sub	x9, x3, x9
	WORD $0x7100051f  // cmp	w8, #1
	WORD $0x540000eb  // b.lt	.LBB1_2

LBB1_1:
	WORD $0x3cc10400 // ldr	q0, [x0], #16
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0x71000508 // subs	w8, w8, #1
	WORD $0x4f819000 // fmul	v0.4s, v0.4s, v1.s[0]
	WORD $0x3c810440 // str	q0, [x2], #16
	WORD $0x54ffff61 // b.ne	.LBB1_1

LBB1_2:
	WORD $0x7100053f // cmp	w9, #1
	WORD $0x5400056b // b.lt	.LBB1_12
	WORD $0x92407d28 // and	x8, x9, #0xffffffff
	WORD $0xf100211f // cmp	x8, #8
	WORD $0x54000062 // b.hs	.LBB1_5
	WORD $0xaa1f03ea // mov	x10, xzr
	WORD $0x1400001c // b	.LBB1_10

LBB1_5:
	WORD $0xd37ef50a // lsl	x10, x8, #2
	WORD $0x9100102b // add	x11, x1, #4
	WORD $0xeb0b005f // cmp	x2, x11
	WORD $0x8b0a004c // add	x12, x2, x10
	WORD $0x8b0a000a // add	x10, x0, x10
	WORD $0xfa413180 // ccmp	x12, x1, #0, lo
	WORD $0x1a9f97eb // cset	w11, hi
	WORD $0xeb0c001f // cmp	x0, x12
	WORD $0xfa4a3042 // ccmp	x2, x10, #2, lo
	WORD $0xaa1f03ea // mov	x10, xzr
	WORD $0x54000223 // b.lo	.LBB1_10
	WORD $0x3700020b // tbnz	w11, #0, .LBB1_10
	WORD $0x92400929 // and	x9, x9, #0x7
	WORD $0x4d40c820 // ld1r	{ v0.4s }, [x1]
	WORD $0x9100400b // add	x11, x0, #16
	WORD $0xcb09010a // sub	x10, x8, x9
	WORD $0x9100404c // add	x12, x2, #16
	WORD $0xaa0a03ed // mov	x13, x10

LBB1_8:
	WORD $0xad7f8961 // ldp	q1, q2, [x11, #-16]
	WORD $0xf10021ad // subs	x13, x13, #8
	WORD $0x9100816b // add	x11, x11, #32
	WORD $0x6e20dc21 // fmul	v1.4s, v1.4s, v0.4s
	WORD $0x6e20dc42 // fmul	v2.4s, v2.4s, v0.4s
	WORD $0xad3f8981 // stp	q1, q2, [x12, #-16]
	WORD $0x9100818c // add	x12, x12, #32
	WORD $0x54ffff21 // b.ne	.LBB1_8
	WORD $0xb4000169 // cbz	x9, .LBB1_12

LBB1_10:
	WORD $0xd37ef54b // lsl	x11, x10, #2
	WORD $0xcb0a0108 // sub	x8, x8, x10
	WORD $0x8b0b0049 // add	x9, x2, x11
	WORD $0x8b0b000b // add	x11, x0, x11

LBB1_11:
	WORD $0xbc404560 // ldr	s0, [x11], #4
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0xf1000508 // subs	x8, x8, #1
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0xbc004520 // str	s0, [x9], #4
	WORD $0x54ffff61 // b.ne	.LBB1_11

LBB1_12:
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET

TEXT ·vmul_const(SB), $0-24
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD n+16(FP), R2
	WORD $0xa9bf7bfd  // stp	x29, x30, [sp, #-16]!
	WORD $0x91000c48  // add	x8, x2, #3
	WORD $0xf100005f  // cmp	x2, #0
	WORD $0x910003fd  // mov	x29, sp
	WORD $0x9a82b109  // csel	x9, x8, x2, lt
	WORD $0xd342fd28  // lsr	x8, x9, #2
	WORD $0x927ef529  // and	x9, x9, #0xfffffffffffffffc
	WORD $0xcb090049  // sub	x9, x2, x9
	WORD $0x7100051f  // cmp	w8, #1
	WORD $0x540000eb  // b.lt	.LBB2_2

LBB2_1:
	WORD $0x3dc00000 // ldr	q0, [x0]
	WORD $0xbd400021 // ldr	s1, [x1]
	WORD $0x71000508 // subs	w8, w8, #1
	WORD $0x4f819000 // fmul	v0.4s, v0.4s, v1.s[0]
	WORD $0x3c810400 // str	q0, [x0], #16
	WORD $0x54ffff61 // b.ne	.LBB2_1

LBB2_2:
	WORD $0x7100053f // cmp	w9, #1
	WORD $0x5400026b // b.lt	.LBB2_9
	WORD $0x92407d28 // and	x8, x9, #0xffffffff
	WORD $0xf100211f // cmp	x8, #8
	WORD $0x540000e3 // b.lo	.LBB2_6
	WORD $0x9100102a // add	x10, x1, #4
	WORD $0xeb0a001f // cmp	x0, x10
	WORD $0x540001e2 // b.hs	.LBB2_10
	WORD $0x8b08080a // add	x10, x0, x8, lsl #2
	WORD $0xeb01015f // cmp	x10, x1
	WORD $0x54000189 // b.ls	.LBB2_10

LBB2_6:
	WORD $0xaa1f03e9 // mov	x9, xzr

LBB2_7:
	WORD $0x8b09080a // add	x10, x0, x9, lsl #2
	WORD $0xcb090108 // sub	x8, x8, x9

LBB2_8:
	WORD $0xbd400020 // ldr	s0, [x1]
	WORD $0xbd400141 // ldr	s1, [x10]
	WORD $0xf1000508 // subs	x8, x8, #1
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0xbc004540 // str	s0, [x10], #4
	WORD $0x54ffff61 // b.ne	.LBB2_8

LBB2_9:
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET

LBB2_10:
	WORD $0x9240092a // and	x10, x9, #0x7
	WORD $0x4d40c820 // ld1r	{ v0.4s }, [x1]
	WORD $0x9100400b // add	x11, x0, #16
	WORD $0xcb0a0109 // sub	x9, x8, x10
	WORD $0xaa0903ec // mov	x12, x9

LBB2_11:
	WORD $0xad7f8961 // ldp	q1, q2, [x11, #-16]
	WORD $0xf100218c // subs	x12, x12, #8
	WORD $0x6e21dc01 // fmul	v1.4s, v0.4s, v1.4s
	WORD $0x6e22dc02 // fmul	v2.4s, v0.4s, v2.4s
	WORD $0xad3f8961 // stp	q1, q2, [x11, #-16]
	WORD $0x9100816b // add	x11, x11, #32
	WORD $0x54ffff41 // b.ne	.LBB2_11
	WORD $0xb5fffd4a // cbnz	x10, .LBB2_7
	WORD $0x17fffff1 // b	.LBB2_9

TEXT ·vmul_to(SB), $0-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD c+16(FP), R2
	MOVD n+24(FP), R3
	WORD $0xa9bf7bfd  // stp	x29, x30, [sp, #-16]!
	WORD $0x91000c68  // add	x8, x3, #3
	WORD $0xf100007f  // cmp	x3, #0
	WORD $0x910003fd  // mov	x29, sp
	WORD $0x9a83b109  // csel	x9, x8, x3, lt
	WORD $0xd342fd28  // lsr	x8, x9, #2
	WORD $0x927ef529  // and	x9, x9, #0xfffffffffffffffc
	WORD $0xcb09006a  // sub	x10, x3, x9
	WORD $0x7100051f  // cmp	w8, #1
	WORD $0x540000eb  // b.lt	.LBB3_2

LBB3_1:
	WORD $0x3cc10400 // ldr	q0, [x0], #16
	WORD $0x71000508 // subs	w8, w8, #1
	WORD $0x3cc10421 // ldr	q1, [x1], #16
	WORD $0x6e21dc00 // fmul	v0.4s, v0.4s, v1.4s
	WORD $0x3c810440 // str	q0, [x2], #16
	WORD $0x54ffff61 // b.ne	.LBB3_1

LBB3_2:
	WORD $0x7100055f // cmp	w10, #1
	WORD $0x5400052b // b.lt	.LBB3_12
	WORD $0x92407d48 // and	x8, x10, #0xffffffff
	WORD $0xf100211f // cmp	x8, #8
	WORD $0x54000062 // b.hs	.LBB3_5
	WORD $0xaa1f03e9 // mov	x9, xzr
	WORD $0x14000019 // b	.LBB3_10

LBB3_5:
	WORD $0xcb000049 // sub	x9, x2, x0
	WORD $0xf100813f // cmp	x9, #32
	WORD $0xaa1f03e9 // mov	x9, xzr
	WORD $0x540002a3 // b.lo	.LBB3_10
	WORD $0xcb01004b // sub	x11, x2, x1
	WORD $0xf100817f // cmp	x11, #32
	WORD $0x54000243 // b.lo	.LBB3_10
	WORD $0x9240094a // and	x10, x10, #0x7
	WORD $0x9100400b // add	x11, x0, #16
	WORD $0x9100402c // add	x12, x1, #16
	WORD $0xcb0a0109 // sub	x9, x8, x10
	WORD $0x9100404d // add	x13, x2, #16
	WORD $0xaa0903ee // mov	x14, x9

LBB3_8:
	WORD $0xad7f8d80 // ldp	q0, q3, [x12, #-16]
	WORD $0xf10021ce // subs	x14, x14, #8
	WORD $0xad7f8961 // ldp	q1, q2, [x11, #-16]
	WORD $0x9100816b // add	x11, x11, #32
	WORD $0x9100818c // add	x12, x12, #32
	WORD $0x6e20dc20 // fmul	v0.4s, v1.4s, v0.4s
	WORD $0x6e23dc41 // fmul	v1.4s, v2.4s, v3.4s
	WORD $0xad3f85a0 // stp	q0, q1, [x13, #-16]
	WORD $0x910081ad // add	x13, x13, #32
	WORD $0x54fffee1 // b.ne	.LBB3_8
	WORD $0xb400018a // cbz	x10, .LBB3_12

LBB3_10:
	WORD $0xd37ef52c // lsl	x12, x9, #2
	WORD $0xcb090108 // sub	x8, x8, x9
	WORD $0x8b0c004a // add	x10, x2, x12
	WORD $0x8b0c002b // add	x11, x1, x12
	WORD $0x8b0c000c // add	x12, x0, x12

LBB3_11:
	WORD $0xbc404580 // ldr	s0, [x12], #4
	WORD $0xf1000508 // subs	x8, x8, #1
	WORD $0xbc404561 // ldr	s1, [x11], #4
	WORD $0x1e210800 // fmul	s0, s0, s1
	WORD $0xbc004540 // str	s0, [x10], #4
	WORD $0x54ffff61 // b.ne	.LBB3_11

LBB3_12:
	WORD $0xa8c17bfd // ldp	x29, x30, [sp], #16
	RET

TEXT ·vdot(SB), $8-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD n+16(FP), R2
	WORD $0xa9bf7bfd  // stp	x29, x30, [sp, #-16]!
	WORD $0x91000c48  // add	x8, x2, #3
	WORD $0xf100005f  // cmp	x2, #0
	WORD $0x910003fd  // mov	x29, sp
	WORD $0x9a82b108  // csel	x8, x8, x2, lt
	WORD $0x9342fd0a  // asr	x10, x8, #2
	WORD $0x927ef508  // and	x8, x8, #0xfffffffffffffffc
	WORD $0xcb080049  // sub	x9, x2, x8
	WORD $0x7100055f  // cmp	w10, #1
	WORD $0x5400028b  // b.lt	.LBB4_5
	WORD $0x3cc10400  // ldr	q0, [x0], #16
	WORD $0x71000548  // subs	w8, w10, #1
	WORD $0x3cc10421  // ldr	q1, [x1], #16
	WORD $0x6e21dc00  // fmul	v0.4s, v0.4s, v1.4s
	WORD $0x54000200  // b.eq	.LBB4_6
	WORD $0xb27b7beb  // mov	x11, #68719476704
	WORD $0xaa0103ec  // mov	x12, x1
	WORD $0x8b0a116a  // add	x10, x11, x10, lsl #4
	WORD $0x927c7d4a  // and	x10, x10, #0xffffffff0
	WORD $0x9100414b  // add	x11, x10, #16
	WORD $0x8b0b000a  // add	x10, x0, x11

LBB4_3:
	WORD $0x3cc10401 // ldr	q1, [x0], #16
	WORD $0x71000508 // subs	w8, w8, #1
	WORD $0x3cc10582 // ldr	q2, [x12], #16
	WORD $0x4e21cc40 // fmla	v0.4s, v2.4s, v1.4s
	WORD $0x54ffff81 // b.ne	.LBB4_3
	WORD $0x8b0b0021 // add	x1, x1, x11
	WORD $0xaa0a03e0 // mov	x0, x10
	WORD $0x14000002 // b	.LBB4_6

LBB4_5:
	WORD $0x6f00e400 // movi	v0.2d, #0000000000000000

LBB4_6:
	WORD $0x2f00e401 // movi	d1, #0000000000000000
	WORD $0x5e0c0402 // mov	s2, v0.s[1]
	WORD $0x7100013f // cmp	w9, #0
	WORD $0x1e212801 // fadd	s1, s0, s1
	WORD $0x1e222821 // fadd	s1, s1, s2
	WORD $0x5e140402 // mov	s2, v0.s[2]
	WORD $0x5e1c0400 // mov	s0, v0.s[3]
	WORD $0x1e222821 // fadd	s1, s1, s2
	WORD $0x1e202820 // fadd	s0, s1, s0
	WORD $0x5400056d // b.le	.LBB4_14
	WORD $0x92407d28 // and	x8, x9, #0xffffffff
	WORD $0xf100211f // cmp	x8, #8
	WORD $0x54000062 // b.hs	.LBB4_9
	WORD $0xaa1f03e9 // mov	x9, xzr
	WORD $0x1400001d // b	.LBB4_12

LBB4_9:
	WORD $0x9240092a // and	x10, x9, #0x7
	WORD $0x9100400b // add	x11, x0, #16
	WORD $0x9100402c // add	x12, x1, #16
	WORD $0xcb0a0109 // sub	x9, x8, x10
	WORD $0xaa0903ed // mov	x13, x9

LBB4_10:
	WORD $0xad7f9181 // ldp	q1, q4, [x12, #-16]
	WORD $0xf10021ad // subs	x13, x13, #8
	WORD $0xad7f8d62 // ldp	q2, q3, [x11, #-16]
	WORD $0x9100816b // add	x11, x11, #32
	WORD $0x9100818c // add	x12, x12, #32
	WORD $0x6e21dc41 // fmul	v1.4s, v2.4s, v1.4s
	WORD $0x5e0c0422 // mov	s2, v1.s[1]
	WORD $0x1e212800 // fadd	s0, s0, s1
	WORD $0x5e140425 // mov	s5, v1.s[2]
	WORD $0x5e1c0421 // mov	s1, v1.s[3]
	WORD $0x1e222800 // fadd	s0, s0, s2
	WORD $0x6e24dc62 // fmul	v2.4s, v3.4s, v4.4s
	WORD $0x1e252800 // fadd	s0, s0, s5
	WORD $0x5e140443 // mov	s3, v2.s[2]
	WORD $0x1e212800 // fadd	s0, s0, s1
	WORD $0x5e0c0441 // mov	s1, v2.s[1]
	WORD $0x1e222800 // fadd	s0, s0, s2
	WORD $0x1e212800 // fadd	s0, s0, s1
	WORD $0x5e1c0441 // mov	s1, v2.s[3]
	WORD $0x1e232800 // fadd	s0, s0, s3
	WORD $0x1e212800 // fadd	s0, s0, s1
	WORD $0x54fffd61 // b.ne	.LBB4_10
	WORD $0xb400014a // cbz	x10, .LBB4_14

LBB4_12:
	WORD $0xd37ef52b // lsl	x11, x9, #2
	WORD $0xcb090108 // sub	x8, x8, x9
	WORD $0x8b0b002a // add	x10, x1, x11
	WORD $0x8b0b000b // add	x11, x0, x11

LBB4_13:
	WORD $0xbc404561 // ldr	s1, [x11], #4
	WORD $0xf1000508 // subs	x8, x8, #1
	WORD $0xbc404542 // ldr	s2, [x10], #4
	WORD $0x1f020020 // fmadd	s0, s1, s2, s0
	WORD $0x54ffff81 // b.ne	.LBB4_13

LBB4_14:
	WORD  $0xa8c17bfd       // ldp	x29, x30, [sp], #16
	FMOVS F0, result+24(FP)
	RET

TEXT ·veuclidean(SB), $8-32
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD n+16(FP), R2
	WORD $0xa9bf7bfd  // stp	x29, x30, [sp, #-16]!
	WORD $0x910003fd  // mov	x29, sp
	WORD $0xd10043ff  // sub	sp, sp, #16
	WORD $0x91000c48  // add	x8, x2, #3
	WORD $0xf100005f  // cmp	x2, #0
	WORD $0x9a82b108  // csel	x8, x8, x2, lt
	WORD $0x9342fd0a  // asr	x10, x8, #2
	WORD $0x927ef508  // and	x8, x8, #0xfffffffffffffffc
	WORD $0xcb080049  // sub	x9, x2, x8
	WORD $0x7100055f  // cmp	w10, #1
	WORD $0x540002cb  // b.lt	.LBB5_5
	WORD $0x3cc10400  // ldr	q0, [x0], #16
	WORD $0x71000548  // subs	w8, w10, #1
	WORD $0x3cc10421  // ldr	q1, [x1], #16
	WORD $0x4ea1d400  // fsub	v0.4s, v0.4s, v1.4s
	WORD $0x6e20dc00  // fmul	v0.4s, v0.4s, v0.4s
	WORD $0x54000220  // b.eq	.LBB5_6
	WORD $0xb27b7beb  // mov	x11, #68719476704
	WORD $0xaa0103ec  // mov	x12, x1
	WORD $0x8b0a116a  // add	x10, x11, x10, lsl #4
	WORD $0x927c7d4a  // and	x10, x10, #0xffffffff0
	WORD $0x9100414b  // add	x11, x10, #16
	WORD $0x8b0b000a  // add	x10, x0, x11

LBB5_3:
	WORD $0x3cc10401 // ldr	q1, [x0], #16
	WORD $0x71000508 // subs	w8, w8, #1
	WORD $0x3cc10582 // ldr	q2, [x12], #16
	WORD $0x4ea2d421 // fsub	v1.4s, v1.4s, v2.4s
	WORD $0x4e21cc20 // fmla	v0.4s, v1.4s, v1.4s
	WORD $0x54ffff61 // b.ne	.LBB5_3
	WORD $0x8b0b0021 // add	x1, x1, x11
	WORD $0xaa0a03e0 // mov	x0, x10
	WORD $0x14000002 // b	.LBB5_6

LBB5_5:
	WORD $0x6f00e400 // movi	v0.2d, #0000000000000000

LBB5_6:
	WORD $0x7e30d801 // faddp	s1, v0.2s
	WORD $0x5e140402 // mov	s2, v0.s[2]
	WORD $0x7100013f // cmp	w9, #0
	WORD $0x5e1c0400 // mov	s0, v0.s[3]
	WORD $0x1e212841 // fadd	s1, s2, s1
	WORD $0x1e212800 // fadd	s0, s0, s1
	WORD $0xbd000be0 // str	s0, [sp, #8]
	WORD $0x540005ed // b.le	.LBB5_15
	WORD $0x92407d28 // and	x8, x9, #0xffffffff
	WORD $0xf100211f // cmp	x8, #8
	WORD $0x54000062 // b.hs	.LBB5_9
	WORD $0xaa1f03e9 // mov	x9, xzr
	WORD $0x1400001f // b	.LBB5_12

LBB5_9:
	WORD $0x9240092a // and	x10, x9, #0x7
	WORD $0x9100400b // add	x11, x0, #16
	WORD $0x9100402c // add	x12, x1, #16
	WORD $0xcb0a0109 // sub	x9, x8, x10
	WORD $0xaa0903ed // mov	x13, x9

LBB5_10:
	WORD $0xad7f9181 // ldp	q1, q4, [x12, #-16]
	WORD $0xf10021ad // subs	x13, x13, #8
	WORD $0xad7f8d62 // ldp	q2, q3, [x11, #-16]
	WORD $0x9100816b // add	x11, x11, #32
	WORD $0x9100818c // add	x12, x12, #32
	WORD $0x4ea1d441 // fsub	v1.4s, v2.4s, v1.4s
	WORD $0x6e21dc21 // fmul	v1.4s, v1.4s, v1.4s
	WORD $0x5e0c0422 // mov	s2, v1.s[1]
	WORD $0x1e212800 // fadd	s0, s0, s1
	WORD $0x5e140425 // mov	s5, v1.s[2]
	WORD $0x5e1c0421 // mov	s1, v1.s[3]
	WORD $0x1e222800 // fadd	s0, s0, s2
	WORD $0x4ea4d462 // fsub	v2.4s, v3.4s, v4.4s
	WORD $0x1e252800 // fadd	s0, s0, s5
	WORD $0x6e22dc42 // fmul	v2.4s, v2.4s, v2.4s
	WORD $0x1e212800 // fadd	s0, s0, s1
	WORD $0x5e0c0441 // mov	s1, v2.s[1]
	WORD $0x5e140443 // mov	s3, v2.s[2]
	WORD $0x1e222800 // fadd	s0, s0, s2
	WORD $0x1e212800 // fadd	s0, s0, s1
	WORD $0x5e1c0441 // mov	s1, v2.s[3]
	WORD $0x1e232800 // fadd	s0, s0, s3
	WORD $0x1e212800 // fadd	s0, s0, s1
	WORD $0x54fffd21 // b.ne	.LBB5_10
	WORD $0xb400016a // cbz	x10, .LBB5_14

LBB5_12:
	WORD $0xd37ef52b // lsl	x11, x9, #2
	WORD $0xcb090108 // sub	x8, x8, x9
	WORD $0x8b0b002a // add	x10, x1, x11
	WORD $0x8b0b000b // add	x11, x0, x11

LBB5_13:
	WORD $0xbc404561 // ldr	s1, [x11], #4
	WORD $0xf1000508 // subs	x8, x8, #1
	WORD $0xbc404542 // ldr	s2, [x10], #4
	WORD $0x1e223821 // fsub	s1, s1, s2
	WORD $0x1f010020 // fmadd	s0, s1, s1, s0
	WORD $0x54ffff61 // b.ne	.LBB5_13

LBB5_14:
	WORD $0xbd000be0 // str	s0, [sp, #8]

LBB5_15:
	WORD  $0xfd4007e0       // ldr	d0, [sp, #8]
	WORD  $0x2ea1f800       // fsqrt	v0.2s, v0.2s
	WORD  $0x910043ff       // add	sp, sp, #16
	WORD  $0xa8c17bfd       // ldp	x29, x30, [sp], #16
	FMOVS F0, result+24(FP)
	RET

TEXT ·vmat_mul(SB), $0-48
	MOVD a+0(FP), R0
	MOVD b+8(FP), R1
	MOVD c+16(FP), R2
	MOVD m+24(FP), R3
	MOVD n+32(FP), R4
	MOVD k+40(FP), R5
	WORD $0xf100047f  // cmp	x3, #1
	WORD $0x54000b2b  // b.lt	.LBB6_15
	WORD $0xf100049f  // cmp	x4, #1
	WORD $0x54000aeb  // b.lt	.LBB6_15
	WORD $0xf10004bf  // cmp	x5, #1
	WORD $0x54000aab  // b.lt	.LBB6_15
	WORD $0xa9bc7bfd  // stp	x29, x30, [sp, #-64]!
	WORD $0x9b047cab  // mul	x11, x5, x4
	WORD $0xd37ef4a9  // lsl	x9, x5, #2
	WORD $0xd37ef48a  // lsl	x10, x4, #2
	WORD $0xaa1f03e8  // mov	x8, xzr
	WORD $0x927df0ae  // and	x14, x5, #0xfffffffffffffff8
	WORD $0x9100402f  // add	x15, x1, #16
	WORD $0x8b09004c  // add	x12, x2, x9
	WORD $0x8b0a000d  // add	x13, x0, x10
	WORD $0x91004050  // add	x16, x2, #16
	WORD $0xaa0203f1  // mov	x17, x2
	WORD $0xf9000bf7  // str	x23, [sp, #16]
	WORD $0x910003fd  // mov	x29, sp
	WORD $0xa90257f6  // stp	x22, x21, [sp, #32]
	WORD $0x8b0b082b  // add	x11, x1, x11, lsl #2
	WORD $0xa9034ff4  // stp	x20, x19, [sp, #48]
	WORD $0x14000006  // b	.LBB6_5

LBB6_4:
	WORD $0x91000508 // add	x8, x8, #1
	WORD $0x8b090210 // add	x16, x16, x9
	WORD $0x8b090231 // add	x17, x17, x9
	WORD $0xeb03011f // cmp	x8, x3
	WORD $0x54000760 // b.eq	.LBB6_14

LBB6_5:
	WORD $0x9b087d26 // mul	x6, x9, x8
	WORD $0xaa1f03f2 // mov	x18, xzr
	WORD $0x9b087d47 // mul	x7, x10, x8
	WORD $0x9b047d15 // mul	x21, x8, x4
	WORD $0x8b060053 // add	x19, x2, x6
	WORD $0x8b060186 // add	x6, x12, x6
	WORD $0x8b0701b4 // add	x20, x13, x7
	WORD $0x8b070007 // add	x7, x0, x7
	WORD $0xeb14027f // cmp	x19, x20
	WORD $0xaa0f03f4 // mov	x20, x15
	WORD $0xfa4630e2 // ccmp	x7, x6, #2, lo
	WORD $0x1a9f27e7 // cset	w7, lo
	WORD $0xeb0b027f // cmp	x19, x11
	WORD $0xaa0103f3 // mov	x19, x1
	WORD $0xfa4130c0 // ccmp	x6, x1, #0, lo
	WORD $0x8b150806 // add	x6, x0, x21, lsl #2
	WORD $0xfa409928 // ccmp	x9, #0, #8, ls
	WORD $0x1a9fa4e7 // csinc	w7, w7, wzr, ge
	WORD $0x14000006 // b	.LBB6_7

LBB6_6:
	WORD $0x91000652 // add	x18, x18, #1
	WORD $0x8b090294 // add	x20, x20, x9
	WORD $0x8b090273 // add	x19, x19, x9
	WORD $0xeb04025f // cmp	x18, x4
	WORD $0x54fffc80 // b.eq	.LBB6_4

LBB6_7:
	WORD $0xf10020bf // cmp	x5, #8
	WORD $0x1a9f24f5 // csinc	w21, w7, wzr, hs
	WORD $0x36000075 // tbz	w21, #0, .LBB6_9
	WORD $0xaa1f03f7 // mov	x23, xzr
	WORD $0x14000012 // b	.LBB6_12

LBB6_9:
	WORD $0x8b1208d5 // add	x21, x6, x18, lsl #2
	WORD $0xaa1003f6 // mov	x22, x16
	WORD $0xaa1403f7 // mov	x23, x20
	WORD $0x4d40caa0 // ld1r	{ v0.4s }, [x21]
	WORD $0xaa0e03f5 // mov	x21, x14

LBB6_10:
	WORD $0xad7f92c1 // ldp	q1, q4, [x22, #-16]
	WORD $0xf10022b5 // subs	x21, x21, #8
	WORD $0xad7f8ee2 // ldp	q2, q3, [x23, #-16]
	WORD $0x910082f7 // add	x23, x23, #32
	WORD $0x4e20cc41 // fmla	v1.4s, v2.4s, v0.4s
	WORD $0x4e20cc64 // fmla	v4.4s, v3.4s, v0.4s
	WORD $0xad3f92c1 // stp	q1, q4, [x22, #-16]
	WORD $0x910082d6 // add	x22, x22, #32
	WORD $0x54ffff01 // b.ne	.LBB6_10
	WORD $0xeb0501df // cmp	x14, x5
	WORD $0xaa0e03f7 // mov	x23, x14
	WORD $0x54fffcc0 // b.eq	.LBB6_6

LBB6_12:
	WORD $0xd37ef6f6 // lsl	x22, x23, #2
	WORD $0xcb1700b7 // sub	x23, x5, x23
	WORD $0x8b160275 // add	x21, x19, x22
	WORD $0x8b160236 // add	x22, x17, x22

LBB6_13:
	WORD $0xbc7278c0 // ldr	s0, [x6, x18, lsl #2]
	WORD $0xbc4046a1 // ldr	s1, [x21], #4
	WORD $0xbd4002c2 // ldr	s2, [x22]
	WORD $0xf10006f7 // subs	x23, x23, #1
	WORD $0x1f010800 // fmadd	s0, s0, s1, s2
	WORD $0xbc0046c0 // str	s0, [x22], #4
	WORD $0x54ffff41 // b.ne	.LBB6_13
	WORD $0x17ffffda // b	.LBB6_6

LBB6_14:
	WORD $0xa9434ff4 // ldp	x20, x19, [sp, #48]
	WORD $0xf9400bf7 // ldr	x23, [sp, #16]
	WORD $0xa94257f6 // ldp	x22, x21, [sp, #32]
	WORD $0xa8c47bfd // ldp	x29, x30, [sp], #64
	RET
