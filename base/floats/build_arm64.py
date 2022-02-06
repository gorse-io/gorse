import os
import re


class Line:

    def __init__(self, line_number, label, asm, binary):
        self.line_number = line_number
        self.label = label
        self.asm = asm
        self.binary = binary

    def __str__(self) -> str:
        plan9asm = ''
        if self.label is not None:
            plan9asm += self.label + ':\n'
        if self.asm == 'ret':
            plan9asm += '\tRET'
        else:
            plan9asm += '\tWORD $0x' + self.binary + ' // ' + self.asm
        return plan9asm


os.system("clang -O3 -S -mno-red-zone -mstackrealign -mllvm -inline-threshold=1000 \
    -fno-asynchronous-unwind-tables -fno-exceptions \
    -fno-rtti -c src/floats_arm64.c -o src/floats_arm64.s")

os.system("clang -O3 -mno-red-zone -mstackrealign -mllvm -inline-threshold=1000 \
    -fno-asynchronous-unwind-tables -fno-exceptions \
    -fno-rtti -c src/floats_arm64.c -o src/floats_arm64.o")

os.system('objdump -d src/floats_arm64.o > src/floats_arm64.txt')

with open("src/floats_arm64.s") as f:
    precedures = dict()
    precedure_name = ''
    line_number = 0
    has_label = False
    for line in f:
        line = line.rstrip()
        if re.match(r'^\s+\..+$', line):
            pass
        elif re.match(r'^\w+:.+$', line):
            precedure_name = line.split(':')[0]
            precedures[precedure_name] = []
            line_number = 0
        elif re.match(r'^\.\w+:.+$', line):
            label = line.split(':')[0]
            label = label[1:]
            has_label = True
            precedures[precedure_name].append(Line(line_number, label, None, None))
        elif re.match(r'^\s+\w+.+$', line):
            asm = line.split('//')[0]
            asm = asm.strip()
            if has_label:
                precedures[precedure_name][line_number].asm = asm
                has_label = False
            else:
                precedures[precedure_name].append(Line(line_number, None, asm, None))
            line_number += 1

with open("src/floats_arm64.txt") as f:
    precedure_name = ''
    line_number = 0
    for line in f:
        line = line.strip()
        if re.match(r'^\w+\s+<\w+>:$', line):
            precedure_name = line.split("<")[1].split('>')[0]
            line_number = 0
        elif re.match(r'^\w+:\s+\w+\s+.+$', line):
            data = line.split(':')[1].strip().split()[0]
            precedures[precedure_name][line_number].binary = data
            line_number += 1

o = open('floats_arm64.s', 'w')
o.write("""//+build !noasm !appengine
// AUTO-GENERATED -- DO NOT EDIT
""")

with open("floats_arm64.go") as f:
    for line in f:
        line = line.strip()
        if line.startswith('//'):
            continue
        elif re.match(r'func \w+\(.+\)', line):
            name = line.split('func ')[1].split('(')[0]
            args = line.split('(')[1].split('unsafe.Pointer')[0].split(',')
            args = [v.strip() for v in args]
            o.write('\n')
            o.write('TEXT ·%s(SB), $0-%d\n' % (name, len(args)*8))
            for (i, arg) in enumerate(args):
                o.write("\tMOVD %s+%d(FP), R%d\n" % (arg, i*8, i))
            p = precedures[name]
            for line in p:
                o.write(str(line) + '\n')

os.remove("src/floats_arm64.txt")
os.remove("src/floats_arm64.s")
os.remove("src/floats_arm64.o")
