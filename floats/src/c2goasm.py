import re

if __name__ == '__main__':
    f = open('internal_avx2.s', 'r')
    outs = []
    for line in f:
        line = line.replace('#', '//')
        if line[0:3] == '.LB':
            # Label
            outs.append(re.sub(r'\.(\w+)', r'\1', line))
        elif line[0:2] == '//':
            # Comment line
            outs.append(line)
        elif line[0] == '\t':
            if line[1] != '.':
                out = ''
                # Replace registers
                line = re.sub(r'%r(\d+)d?', r'R\1', line)
                line = re.sub(r'%(r|e)(\w\w)', r'\2', line)
                line = re.sub(r'%(x|y)mm(\d+)', r'\1\2', line)
                # Transform label
                line = re.sub(r'\.(\w+)', r'\1', line)
                # Transform addressing
                line = re.sub(r'\((\w+),(\w+),(\d)\)', r'(\1)(\2*\3)', line)
                line = re.sub(r'\(,(\w+),(\d)\)', r'(\1*\2)', line)
                # Transform cmp
                line = re.sub(r'(cmp\w)\t([\w\d\$]+), (\w+)', r'\1\t\3, \2', line)
                # Transform retq
                line = re.sub(r'retq', r'ret', line)
                outs.append(line.upper())
    with open('out.s', 'w') as f:
        f.writelines(outs)
