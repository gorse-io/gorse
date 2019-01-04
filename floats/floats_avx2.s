//+build avx2

TEXT    ·_MulConstTo+0(SB),$0-64
    MOVQ    a+0(FP), CX
    MOVQ    c+24(FP), X1
    MOVQ    dst+32(FP), R8
    MOVQ    len+56(FP), R9
// %bb.0:
    PUSHQ	SI
    PUSHQ	DI
    PUSHQ	BX
    MOVL	R9, SI
    SARL	$31, SI
    SHRL	$30, SI
    ADDL	R9, SI
    MOVL	SI, AX
    ANDL	$-4, AX
    MOVL	R9, R10
    SUBL	AX, R10
    VBROADCASTSD	X1, Y0
    CMPL	R9, $4
    JL	LBB0_8
// %bb.1:
    SARL	$2, SI
    LEAL	-1(SI), R11
    MOVL	SI, R9
    ANDL	$3, R9
    MOVQ	R8, DX
    MOVQ	CX, AX
    CMPL	R11, $3
    JB	    LBB0_4
// %bb.2:
    MOVL	R9, DI
    SUBL	SI, DI
    MOVQ	R8, DX
    MOVQ	CX, AX
LBB0_3:                                // =>This Inner Loop Header: Depth=1
    VMULPD	(AX), Y0, Y2
    VMOVUPD	Y2, (DX)
    VMULPD	32(AX), Y0, Y2
    VMOVUPD	Y2, 32(DX)
    VMULPD	64(AX), Y0, Y2
    VMOVUPD	Y2, 64(DX)
    VMULPD	96(AX), Y0, Y2
    VMOVUPD	Y2, 96(DX)
    SUBQ	$-128, AX
    SUBQ	$-128, DX
    ADDL	$4, DI
    JNE	LBB0_3
LBB0_4:
    LEAQ	4(R11*4), BX
    LEAQ	(R11*4), DI
    TESTL	R9, R9
    JE	LBB0_7
// %bb.5:
    NEGL	R9
    XORL	SI, SI
LBB0_6:                                // =>This Inner Loop Header: Depth=1
    VMULPD	(AX)(SI*1), Y0, Y2
    VMOVUPD	Y2, (DX)(SI*1)
    ADDQ	$32, SI
    ADDL	$1, R9
    JNE	LBB0_6
LBB0_7:
    LEAQ	(CX)(DI*8), CX
    ADDQ	$32, CX
    LEAQ	(R8)(BX*8), R8
LBB0_8:
    TESTL	R10, R10
    JLE	LBB0_23
// %bb.9:
    LEAL	-1(R10), AX
    LEAQ	1(AX), R9
    CMPQ	R9, $15
    JBE	LBB0_10
// %bb.18:
    LEAQ	(CX)(AX*8), DX
    ADDQ	$8, DX
    CMPQ	R8, DX
    JAE	LBB0_20
// %bb.19:
    LEAQ	(R8)(AX*8), AX
    ADDQ	$8, AX
    CMPQ	CX, AX
    JAE	LBB0_20
LBB0_10:
    XORL	R9, R9
    MOVQ	R8, DX
    MOVQ	CX, AX
LBB0_11:
    MOVL	R10, CX
    SUBL	R9, CX
    MOVL	R9, R8
    NOTL	R8
    ADDL	R10, R8
    ANDL	$3, CX
    JE	LBB0_15
// %bb.12:
    NEGL	CX
    XORL	DI, DI
LBB0_13:                               // =>This Inner Loop Header: Depth=1
    VMULSD	(AX), X1, X0
    VMOVSD	X0, (DX)
    ADDQ	$8, AX
    ADDQ	$8, DX
    ADDL	$-1, DI
    CMPL	CX, DI
    JNE	LBB0_13
// %bb.14:
    SUBL	DI, R9
LBB0_15:
    CMPL	R8, $3
    JB	LBB0_23
// %bb.16:
    SUBL	R9, R10
    XORL	CX, CX
LBB0_17:                               // =>This Inner Loop Header: Depth=1
    VMULSD	(AX)(CX*8), X1, X0
    VMOVSD	X0, (DX)(CX*8)
    VMULSD	8(AX)(CX*8), X1, X0
    VMOVSD	X0, 8(DX)(CX*8)
    VMULSD	16(AX)(CX*8), X1, X0
    VMOVSD	X0, 16(DX)(CX*8)
    VMULSD	24(AX)(CX*8), X1, X0
    VMOVSD	X0, 24(DX)(CX*8)
    ADDQ	$4, CX
    CMPL	R10, CX
    JNE	LBB0_17
    JMP	LBB0_23
LBB0_20:
    MOVL	R10, R11
    ANDL	$15, R11
    SUBQ	R11, R9
    LEAQ	(R8)(R9*8), DX
    LEAQ	(CX)(R9*8), AX
    XORL	SI, SI
LBB0_21:                               // =>This Inner Loop Header: Depth=1
    VMULPD	(CX)(SI*8), Y0, Y2
    VMULPD	32(CX)(SI*8), Y0, Y3
    VMULPD	64(CX)(SI*8), Y0, Y4
    VMULPD	96(CX)(SI*8), Y0, Y5
    VMOVUPD	Y2, (R8)(SI*8)
    VMOVUPD	Y3, 32(R8)(SI*8)
    VMOVUPD	Y4, 64(R8)(SI*8)
    VMOVUPD	Y5, 96(R8)(SI*8)
    ADDQ	$16, SI
    CMPQ	R9, SI
    JNE	LBB0_21
// %bb.22:
    TESTL	R11, R11
    JNE	LBB0_11
LBB0_23:
    POPQ	BX
    POPQ	DI
    POPQ	SI
    VZEROUPPER
    RET

TEXT    ·_MulConstAddTo+0(SB),$0-64
    MOVQ    a+0(FP), CX
    MOVQ    c+24(FP), X1
    MOVQ    dst+32(FP), R8
    MOVQ    len+56(FP), R9
// %bb.0:
	PUSHQ	SI
	PUSHQ	DI
	PUSHQ	BX
	MOVL	R9, DI
	SARL	$31, DI
	SHRL	$30, DI
	ADDL	R9, DI
	MOVL	DI, AX
	ANDL	$-4, AX
	MOVL	R9, R10
	SUBL	AX, R10
	VBROADCASTSD	X1, Y0
	CMPL	R9, $4
	JL	LBB1_8
// %bb.1:
	SARL	$2, DI
	LEAL	-1(DI), R11
	MOVL	DI, R9
	ANDL	$3, R9
	MOVQ	R8, DX
	MOVQ	CX, AX
	CMPL	R11, $3
	JB	LBB1_4
// %bb.2:
	MOVL	R9, SI
	SUBL	DI, SI
	MOVQ	R8, DX
	MOVQ	CX, AX
LBB1_3:                                // =>This Inner Loop Header: Depth=1
	VMOVAPD	(AX), Y2
	VFMADD213PD	(DX), Y0, Y2 // YMM2 = (YMM0 * YMM2) + MEM
	VMOVUPD	Y2, (DX)
	VMOVAPD	32(AX), Y2
	VFMADD213PD	32(DX), Y0, Y2 // YMM2 = (YMM0 * YMM2) + MEM
	VMOVUPD	Y2, 32(DX)
	VMOVAPD	64(AX), Y2
	VFMADD213PD	64(DX), Y0, Y2 // YMM2 = (YMM0 * YMM2) + MEM
	VMOVUPD	Y2, 64(DX)
	VMOVAPD	96(AX), Y2
	VFMADD213PD	96(DX), Y0, Y2 // YMM2 = (YMM0 * YMM2) + MEM
	VMOVUPD	Y2, 96(DX)
	SUBQ	$-128, AX
	SUBQ	$-128, DX
	ADDL	$4, SI
	JNE	LBB1_3
LBB1_4:
	LEAQ	4(R11*4), BX
	LEAQ	(R11*4), DI
	TESTL	R9, R9
	JE	LBB1_7
// %bb.5:
	NEGL	R9
	XORL	SI, SI
LBB1_6:                                // =>This Inner Loop Header: Depth=1
	VMOVAPD	(AX)(SI*1), Y2
	VFMADD213PD	(DX)(SI*1), Y0, Y2 // YMM2 = (YMM0 * YMM2) + MEM
	VMOVUPD	Y2, (DX)(SI*1)
	ADDQ	$32, SI
	ADDL	$1, R9
	JNE	LBB1_6
LBB1_7:
	LEAQ	(CX)(DI*8), CX
	ADDQ	$32, CX
	LEAQ	(R8)(BX*8), R8
LBB1_8:
	TESTL	R10, R10
	JLE	LBB1_23
// %bb.9:
	LEAL	-1(R10), AX
	LEAQ	1(AX), R9
	CMPQ	R9, $15
	JBE	LBB1_10
// %bb.18:
	LEAQ	(CX)(AX*8), DX
	ADDQ	$8, DX
	CMPQ	R8, DX
	JAE	LBB1_20
// %bb.19:
	LEAQ	(R8)(AX*8), AX
	ADDQ	$8, AX
	CMPQ	CX, AX
	JAE	LBB1_20
LBB1_10:
	XORL	R9, R9
	MOVQ	R8, DX
	MOVQ	CX, AX
LBB1_11:
	MOVL	R10, SI
	SUBL	R9, SI
	MOVL	R9, CX
	NOTL	CX
	ADDL	R10, CX
	ANDL	$3, SI
	JE	LBB1_15
// %bb.12:
	NEGL	SI
	XORL	DI, DI
LBB1_13:                               // =>This Inner Loop Header: Depth=1
	VMULSD	(AX), X1, X0
	VADDSD	(DX), X0, X0
	VMOVSD	X0, (DX)
	ADDQ	$8, AX
	ADDQ	$8, DX
	ADDL	$-1, DI
	CMPL	SI, DI
	JNE	LBB1_13
// %bb.14:
	SUBL	DI, R9
LBB1_15:
	CMPL	CX, $3
	JB	LBB1_23
// %bb.16:
	SUBL	R9, R10
	XORL	CX, CX
LBB1_17:                               // =>This Inner Loop Header: Depth=1
	VMULSD	(AX)(CX*8), X1, X0
	VADDSD	(DX)(CX*8), X0, X0
	VMOVSD	X0, (DX)(CX*8)
	VMULSD	8(AX)(CX*8), X1, X0
	VADDSD	8(DX)(CX*8), X0, X0
	VMOVSD	X0, 8(DX)(CX*8)
	VMULSD	16(AX)(CX*8), X1, X0
	VADDSD	16(DX)(CX*8), X0, X0
	VMOVSD	X0, 16(DX)(CX*8)
	VMULSD	24(AX)(CX*8), X1, X0
	VADDSD	24(DX)(CX*8), X0, X0
	VMOVSD	X0, 24(DX)(CX*8)
	ADDQ	$4, CX
	CMPL	R10, CX
	JNE	LBB1_17
	JMP	LBB1_23
LBB1_20:
	MOVL	R10, SI
	ANDL	$15, SI
	SUBQ	SI, R9
	LEAQ	(R8)(R9*8), DX
	LEAQ	(CX)(R9*8), AX
	XORL	DI, DI
LBB1_21:                               // =>This Inner Loop Header: Depth=1
	VMULPD	(CX)(DI*8), Y0, Y2
	VMULPD	32(CX)(DI*8), Y0, Y3
	VMULPD	64(CX)(DI*8), Y0, Y4
	VMULPD	96(CX)(DI*8), Y0, Y5
	VADDPD	(R8)(DI*8), Y2, Y2
	VADDPD	32(R8)(DI*8), Y3, Y3
	VADDPD	64(R8)(DI*8), Y4, Y4
	VADDPD	96(R8)(DI*8), Y5, Y5
	VMOVUPD	Y2, (R8)(DI*8)
	VMOVUPD	Y3, 32(R8)(DI*8)
	VMOVUPD	Y4, 64(R8)(DI*8)
	VMOVUPD	Y5, 96(R8)(DI*8)
	ADDQ	$16, DI
	CMPQ	R9, DI
	JNE	LBB1_21
// %bb.22:
	TESTL	SI, SI
	JNE	LBB1_11
LBB1_23:
	POPQ	BX
	POPQ	DI
	POPQ	SI
	VZEROUPPER
	RET

TEXT    ·_Dot+0(SB),$0-64
    MOVQ    a+0(FP), CX
    MOVQ    b+24(FP), DX
    MOVQ    len+48(FP), R8
// %bb.0:
	CMPL	R8, $4
	JL	LBB2_4
// %bb.1:
	VMOVAPD	(CX), Y0
	VMULPD	(DX), Y0, Y0
	CMPL	R8, $8
	JL	LBB2_4
// %bb.2:
	MOVL	R8, R9
	SARL	$31, R9
	SHRL	$30, R9
	ADDL	R8, R9
	SARL	$2, R9
	MOVL	$1, R8
	MOVL	$32, AX
LBB2_3:                                // =>This Inner Loop Header: Depth=1
	VMOVAPD	(CX,AX), Y1
	VFMADD231PD	(DX,AX), Y1, Y0 // YMM0 = (YMM1 * MEM) + YMM0
	ADDL	$1, R8
	ADDQ	$32, AX
	CMPL	R8, R9
	JL	LBB2_3
LBB2_4:
	VHADDPD	Y0, Y0, Y0
	VEXTRACTF128	$1, Y0, X1
	VADDPD	X0, X1, X0
	VZEROUPPER
	RET