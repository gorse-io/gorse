//go:build amd64
// +build amd64
// AUTO-GENERATED BY C2GOASM -- DO NOT EDIT

TEXT ·__mm256_mul_const_add_to(SB), $0-32

	MOVQ a+0(FP), DI
	MOVQ b+8(FP), SI
	MOVQ c+16(FP), DX
	MOVQ n+24(FP), CX

	WORD $0x8948; BYTE $0xc8               // mov    rax, rcx
	LONG $0x3ff8c148                       // sar    rax, 63
	LONG $0x3de8c148                       // shr    rax, 61
	WORD $0x0148; BYTE $0xc8               // add    rax, rcx
	WORD $0x8948; BYTE $0xc3               // mov    rbx, rax
	LONG $0x03fbc148                       // sar    rbx, 3
	LONG $0xf8e08348                       // and    rax, -8
	WORD $0x2948; BYTE $0xc1               // sub    rcx, rax
	WORD $0xdb85                           // test    ebx, ebx
	JLE  LBB0_7
	QUAD $0x0007fffffff8b848; WORD $0x0000 // mov    rax, 34359738360
	LONG $0xd8048d4c                       // lea    r8, [rax + 8*rbx]
	WORD $0x2149; BYTE $0xc0               // and    r8, rax
	WORD $0x8941; BYTE $0xd9               // mov    r9d, ebx
	LONG $0x01e18341                       // and    r9d, 1
	WORD $0x8949; BYTE $0xfa               // mov    r10, rdi
	WORD $0x8948; BYTE $0xd0               // mov    rax, rdx
	WORD $0xfb83; BYTE $0x01               // cmp    ebx, 1
	JE   LBB0_4
	WORD $0x8945; BYTE $0xcb               // mov    r11d, r9d
	WORD $0x2941; BYTE $0xdb               // sub    r11d, ebx
	WORD $0x8949; BYTE $0xfa               // mov    r10, rdi
	WORD $0x8948; BYTE $0xd0               // mov    rax, rdx

LBB0_3:
	LONG $0x107cc1c4; BYTE $0x02   // vmovups    ymm0, yword [r10]
	LONG $0x187de2c4; BYTE $0x0e   // vbroadcastss    ymm1, dword [rsi]
	LONG $0xa87de2c4; BYTE $0x08   // vfmadd213ps    ymm1, ymm0, yword [rax]
	LONG $0x0811fcc5               // vmovups    yword [rax], ymm1
	LONG $0x107cc1c4; WORD $0x2042 // vmovups    ymm0, yword [r10 + 32]
	LONG $0x187de2c4; BYTE $0x0e   // vbroadcastss    ymm1, dword [rsi]
	LONG $0xa87de2c4; WORD $0x2048 // vfmadd213ps    ymm1, ymm0, yword [rax + 32]
	LONG $0x4811fcc5; BYTE $0x20   // vmovups    yword [rax + 32], ymm1
	LONG $0x40c28349               // add    r10, 64
	LONG $0x40c08348               // add    rax, 64
	LONG $0x02c38341               // add    r11d, 2
	JNE  LBB0_3

LBB0_4:
	LONG $0x08588d49             // lea    rbx, [r8 + 8]
	WORD $0x8545; BYTE $0xc9     // test    r9d, r9d
	JE   LBB0_6
	LONG $0x107cc1c4; BYTE $0x02 // vmovups    ymm0, yword [r10]
	LONG $0x187de2c4; BYTE $0x0e // vbroadcastss    ymm1, dword [rsi]
	LONG $0xa87de2c4; BYTE $0x08 // vfmadd213ps    ymm1, ymm0, yword [rax]
	LONG $0x0811fcc5             // vmovups    yword [rax], ymm1

LBB0_6:
	LONG $0x873c8d4a // lea    rdi, [rdi + 4*r8]
	LONG $0x20c78348 // add    rdi, 32
	LONG $0x9a148d48 // lea    rdx, [rdx + 4*rbx]

LBB0_7:
	WORD $0xc985             // test    ecx, ecx
	JLE  LBB0_20
	WORD $0x8941; BYTE $0xc8 // mov    r8d, ecx
	LONG $0x1ff88349         // cmp    r8, 31
	JA   LBB0_15
	WORD $0xc031             // xor    eax, eax
	WORD $0xc129             // sub    ecx, eax
	WORD $0x8948; BYTE $0xc3 // mov    rbx, rax
	WORD $0xc1f6; BYTE $0x01 // test    cl, 1
	JNE  LBB0_11
	JMP  LBB0_12

LBB0_15:
	LONG $0x82048d4a             // lea    rax, [rdx + 4*r8]
	LONG $0x871c8d4a             // lea    rbx, [rdi + 4*r8]
	LONG $0x014e8d4c             // lea    r9, [rsi + 1]
	WORD $0x3948; BYTE $0xda     // cmp    rdx, rbx
	LONG $0xd2920f41             // setb    r10b
	WORD $0x3948; BYTE $0xc7     // cmp    rdi, rax
	LONG $0xd3920f41             // setb    r11b
	WORD $0x3949; BYTE $0xd1     // cmp    r9, rdx
	WORD $0x970f; BYTE $0xd3     // seta    bl
	WORD $0x3948; BYTE $0xf0     // cmp    rax, rsi
	LONG $0xd1970f41             // seta    r9b
	WORD $0xc031                 // xor    eax, eax
	WORD $0x8445; BYTE $0xda     // test    r10b, r11b
	JNE  LBB0_10
	WORD $0x2044; BYTE $0xcb     // and    bl, r9b
	JNE  LBB0_10
	WORD $0x8941; BYTE $0xc9     // mov    r9d, ecx
	LONG $0x1fe18341             // and    r9d, 31
	WORD $0x894c; BYTE $0xc0     // mov    rax, r8
	WORD $0x294c; BYTE $0xc8     // sub    rax, r9
	LONG $0x187de2c4; BYTE $0x06 // vbroadcastss    ymm0, dword [rsi]
	WORD $0xdb31                 // xor    ebx, ebx

LBB0_18:
	LONG $0x0c59fcc5; BYTE $0x9f   // vmulps    ymm1, ymm0, yword [rdi + 4*rbx]
	LONG $0x5459fcc5; WORD $0x209f // vmulps    ymm2, ymm0, yword [rdi + 4*rbx + 32]
	LONG $0x5c59fcc5; WORD $0x409f // vmulps    ymm3, ymm0, yword [rdi + 4*rbx + 64]
	LONG $0x6459fcc5; WORD $0x609f // vmulps    ymm4, ymm0, yword [rdi + 4*rbx + 96]
	LONG $0x0c58f4c5; BYTE $0x9a   // vaddps    ymm1, ymm1, yword [rdx + 4*rbx]
	LONG $0x5458ecc5; WORD $0x209a // vaddps    ymm2, ymm2, yword [rdx + 4*rbx + 32]
	LONG $0x5c58e4c5; WORD $0x409a // vaddps    ymm3, ymm3, yword [rdx + 4*rbx + 64]
	LONG $0x6458dcc5; WORD $0x609a // vaddps    ymm4, ymm4, yword [rdx + 4*rbx + 96]
	LONG $0x0c11fcc5; BYTE $0x9a   // vmovups    yword [rdx + 4*rbx], ymm1
	LONG $0x5411fcc5; WORD $0x209a // vmovups    yword [rdx + 4*rbx + 32], ymm2
	LONG $0x5c11fcc5; WORD $0x409a // vmovups    yword [rdx + 4*rbx + 64], ymm3
	LONG $0x6411fcc5; WORD $0x609a // vmovups    yword [rdx + 4*rbx + 96], ymm4
	LONG $0x20c38348               // add    rbx, 32
	WORD $0x3948; BYTE $0xd8       // cmp    rax, rbx
	JNE  LBB0_18
	WORD $0x854d; BYTE $0xc9       // test    r9, r9
	JE   LBB0_20

LBB0_10:
	WORD $0xc129             // sub    ecx, eax
	WORD $0x8948; BYTE $0xc3 // mov    rbx, rax
	WORD $0xc1f6; BYTE $0x01 // test    cl, 1
	JE   LBB0_12

LBB0_11:
	LONG $0x0410fac5; BYTE $0x87 // vmovss    xmm0, dword [rdi + 4*rax]
	LONG $0x0659fac5             // vmulss    xmm0, xmm0, dword [rsi]
	LONG $0x0458fac5; BYTE $0x82 // vaddss    xmm0, xmm0, dword [rdx + 4*rax]
	LONG $0x0411fac5; BYTE $0x82 // vmovss    dword [rdx + 4*rax], xmm0
	LONG $0x01588d48             // lea    rbx, [rax + 1]

LBB0_12:
	LONG $0x01c08348         // add    rax, 1
	WORD $0x3949; BYTE $0xc0 // cmp    r8, rax
	JE   LBB0_20
	WORD $0x2949; BYTE $0xd8 // sub    r8, rbx
	LONG $0x9a048d48         // lea    rax, [rdx + 4*rbx]
	LONG $0x04c08348         // add    rax, 4
	LONG $0x9f0c8d48         // lea    rcx, [rdi + 4*rbx]
	LONG $0x04c18348         // add    rcx, 4
	WORD $0xd231             // xor    edx, edx

LBB0_14:
	LONG $0x4410fac5; WORD $0xfc91 // vmovss    xmm0, dword [rcx + 4*rdx - 4]
	LONG $0x0659fac5               // vmulss    xmm0, xmm0, dword [rsi]
	LONG $0x4458fac5; WORD $0xfc90 // vaddss    xmm0, xmm0, dword [rax + 4*rdx - 4]
	LONG $0x4411fac5; WORD $0xfc90 // vmovss    dword [rax + 4*rdx - 4], xmm0
	LONG $0x0410fac5; BYTE $0x91   // vmovss    xmm0, dword [rcx + 4*rdx]
	LONG $0x0659fac5               // vmulss    xmm0, xmm0, dword [rsi]
	LONG $0x0458fac5; BYTE $0x90   // vaddss    xmm0, xmm0, dword [rax + 4*rdx]
	LONG $0x0411fac5; BYTE $0x90   // vmovss    dword [rax + 4*rdx], xmm0
	LONG $0x02c28348               // add    rdx, 2
	WORD $0x3949; BYTE $0xd0       // cmp    r8, rdx
	JNE  LBB0_14

LBB0_20:
	VZEROUPPER
	RET

TEXT ·__mm256_dot(SB), $0-32

	MOVQ a+0(FP), DI
	MOVQ b+8(FP), SI
	MOVQ n+16(FP), DX
	MOVQ ret+24(FP), CX

	WORD $0x8948; BYTE $0xd3               // mov    rbx, rdx
	LONG $0x3ffbc148                       // sar    rbx, 63
	LONG $0x3debc148                       // shr    rbx, 61
	WORD $0x0148; BYTE $0xd3               // add    rbx, rdx
	WORD $0x8948; BYTE $0xd8               // mov    rax, rbx
	LONG $0x03f8c148                       // sar    rax, 3
	LONG $0xf8e38348                       // and    rbx, -8
	WORD $0x2948; BYTE $0xda               // sub    rdx, rbx
	WORD $0xc085                           // test    eax, eax
	JLE  LBB1_1
	LONG $0x0710fcc5                       // vmovups    ymm0, yword [rdi]
	LONG $0x0659fcc5                       // vmulps    ymm0, ymm0, yword [rsi]
	LONG $0x20c78348                       // add    rdi, 32
	LONG $0x20c68348                       // add    rsi, 32
	WORD $0xf883; BYTE $0x01               // cmp    eax, 1
	JE   LBB1_9
	QUAD $0x0007fffffff0b849; WORD $0x0000 // mov    r8, 34359738352
	LONG $0xc01c8d49                       // lea    rbx, [r8 + 8*rax]
	LONG $0x08c08349                       // add    r8, 8
	WORD $0x2149; BYTE $0xd8               // and    r8, rbx
	LONG $0xff488d44                       // lea    r9d, [rax - 1]
	WORD $0x588d; BYTE $0xfe               // lea    ebx, [rax - 2]
	LONG $0x03e18341                       // and    r9d, 3
	WORD $0xfb83; BYTE $0x03               // cmp    ebx, 3
	JAE  LBB1_16
	WORD $0x8949; BYTE $0xfa               // mov    r10, rdi
	WORD $0x8948; BYTE $0xf0               // mov    rax, rsi
	LONG $0x08588d4d                       // lea    r11, [r8 + 8]
	WORD $0x8545; BYTE $0xc9               // test    r9d, r9d
	JNE  LBB1_6
	JMP  LBB1_8

LBB1_1:
	JMP LBB1_9

LBB1_16:
	LONG $0x01598d45         // lea    r11d, [r9 + 1]
	WORD $0x2941; BYTE $0xc3 // sub    r11d, eax
	WORD $0x8949; BYTE $0xfa // mov    r10, rdi
	WORD $0x8948; BYTE $0xf0 // mov    rax, rsi

LBB1_17:
	LONG $0x107cc1c4; BYTE $0x0a   // vmovups    ymm1, yword [r10]
	LONG $0x107cc1c4; WORD $0x2052 // vmovups    ymm2, yword [r10 + 32]
	LONG $0x107cc1c4; WORD $0x405a // vmovups    ymm3, yword [r10 + 64]
	LONG $0x107cc1c4; WORD $0x6062 // vmovups    ymm4, yword [r10 + 96]
	LONG $0x987de2c4; BYTE $0x08   // vfmadd132ps    ymm1, ymm0, yword [rax]
	LONG $0xb86de2c4; WORD $0x2048 // vfmadd231ps    ymm1, ymm2, yword [rax + 32]
	LONG $0xb865e2c4; WORD $0x4048 // vfmadd231ps    ymm1, ymm3, yword [rax + 64]
	LONG $0xc128fcc5               // vmovaps    ymm0, ymm1
	LONG $0xb85de2c4; WORD $0x6040 // vfmadd231ps    ymm0, ymm4, yword [rax + 96]
	LONG $0x80ea8349               // sub    r10, -128
	LONG $0x80e88348               // sub    rax, -128
	LONG $0x04c38341               // add    r11d, 4
	JNE  LBB1_17
	LONG $0x08588d4d               // lea    r11, [r8 + 8]
	WORD $0x8545; BYTE $0xc9       // test    r9d, r9d
	JE   LBB1_8

LBB1_6:
	WORD $0xf741; BYTE $0xd9 // neg    r9d
	WORD $0xdb31             // xor    ebx, ebx

LBB1_7:
	LONG $0x107cc1c4; WORD $0x1a0c // vmovups    ymm1, yword [r10 + rbx]
	LONG $0xb875e2c4; WORD $0x1804 // vfmadd231ps    ymm0, ymm1, yword [rax + rbx]
	LONG $0x20c38348               // add    rbx, 32
	LONG $0x01c18341               // add    r9d, 1
	JNE  LBB1_7

LBB1_8:
	LONG $0x873c8d4a // lea    rdi, [rdi + 4*r8]
	LONG $0x20c78348 // add    rdi, 32
	LONG $0x9e348d4a // lea    rsi, [rsi + 4*r11]

LBB1_9:
	LONG $0x197de3c4; WORD $0x01c1 // vextractf128    xmm1, ymm0, 1
	LONG $0xc058f0c5               // vaddps    xmm0, xmm1, xmm0
	LONG $0x0579e3c4; WORD $0x03c8 // vpermilpd    xmm1, xmm0, 3
	LONG $0xc158f8c5               // vaddps    xmm0, xmm0, xmm1
	LONG $0xc816fac5               // vmovshdup    xmm1, xmm0
	LONG $0xc158fac5               // vaddss    xmm0, xmm0, xmm1
	LONG $0x0111fac5               // vmovss    dword [rcx], xmm0
	WORD $0xd285                   // test    edx, edx
	JLE  LBB1_15
	WORD $0x8941; BYTE $0xd0       // mov    r8d, edx
	LONG $0xff408d49               // lea    rax, [r8 - 1]
	WORD $0xe283; BYTE $0x03       // and    edx, 3
	LONG $0x03f88348               // cmp    rax, 3
	JAE  LBB1_18
	WORD $0xc031                   // xor    eax, eax
	WORD $0x8548; BYTE $0xd2       // test    rdx, rdx
	JNE  LBB1_13
	JMP  LBB1_15

LBB1_18:
	WORD $0x2949; BYTE $0xd0 // sub    r8, rdx
	WORD $0xc031             // xor    eax, eax

LBB1_19:
	LONG $0x0c10fac5; BYTE $0x87   // vmovss    xmm1, dword [rdi + 4*rax]
	LONG $0x0c59f2c5; BYTE $0x86   // vmulss    xmm1, xmm1, dword [rsi + 4*rax]
	LONG $0xc158fac5               // vaddss    xmm0, xmm0, xmm1
	LONG $0x0111fac5               // vmovss    dword [rcx], xmm0
	LONG $0x4c10fac5; WORD $0x0487 // vmovss    xmm1, dword [rdi + 4*rax + 4]
	LONG $0x4c59f2c5; WORD $0x0486 // vmulss    xmm1, xmm1, dword [rsi + 4*rax + 4]
	LONG $0xc158fac5               // vaddss    xmm0, xmm0, xmm1
	LONG $0x0111fac5               // vmovss    dword [rcx], xmm0
	LONG $0x4c10fac5; WORD $0x0887 // vmovss    xmm1, dword [rdi + 4*rax + 8]
	LONG $0x4c59f2c5; WORD $0x0886 // vmulss    xmm1, xmm1, dword [rsi + 4*rax + 8]
	LONG $0xc158fac5               // vaddss    xmm0, xmm0, xmm1
	LONG $0x0111fac5               // vmovss    dword [rcx], xmm0
	LONG $0x4c10fac5; WORD $0x0c87 // vmovss    xmm1, dword [rdi + 4*rax + 12]
	LONG $0x4c59f2c5; WORD $0x0c86 // vmulss    xmm1, xmm1, dword [rsi + 4*rax + 12]
	LONG $0xc158fac5               // vaddss    xmm0, xmm0, xmm1
	LONG $0x0111fac5               // vmovss    dword [rcx], xmm0
	LONG $0x04c08348               // add    rax, 4
	WORD $0x3949; BYTE $0xc0       // cmp    r8, rax
	JNE  LBB1_19
	WORD $0x8548; BYTE $0xd2       // test    rdx, rdx
	JE   LBB1_15

LBB1_13:
	LONG $0x86348d48 // lea    rsi, [rsi + 4*rax]
	LONG $0x87048d48 // lea    rax, [rdi + 4*rax]
	WORD $0xff31     // xor    edi, edi

LBB1_14:
	LONG $0x0c10fac5; BYTE $0xb8 // vmovss    xmm1, dword [rax + 4*rdi]
	LONG $0x0c59f2c5; BYTE $0xbe // vmulss    xmm1, xmm1, dword [rsi + 4*rdi]
	LONG $0xc158fac5             // vaddss    xmm0, xmm0, xmm1
	LONG $0x0111fac5             // vmovss    dword [rcx], xmm0
	LONG $0x01c78348             // add    rdi, 1
	WORD $0x3948; BYTE $0xfa     // cmp    rdx, rdi
	JNE  LBB1_14

LBB1_15:
	VZEROUPPER
	RET