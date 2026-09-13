#!/usr/bin/env python3
"""MapleMono 字体子集化：把 6MB 的全量字体裁剪到仓库实际用到的字符集（约 280KB）。

字符集来源：webpage/src、docsite、backend 注释等仓库语料 + ASCII + 常用中英文标点。
用户数据中出现语料之外的生僻字时，回退到系统等宽字体（font-family 栈已含 monospace）。

用法（需 fonttools + brotli）：
    pip install fonttools brotli
    python3 scripts/subset-font.py
"""
import glob
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC_FONT = os.path.join(ROOT, 'assets', 'MapleMono-full.woff2')


def collect_chars() -> str:
    chars = set()
    patterns = [
        os.path.join(ROOT, 'src', '**', '*.ts'),
        os.path.join(ROOT, 'src', '**', '*.tsx'),
        os.path.join(ROOT, 'index.html'),
        os.path.join(ROOT, 'public', '*.html'),
        os.path.join(ROOT, '..', 'docsite', '**', '*.md'),
        os.path.join(ROOT, '..', 'backend', '**', '*.go'),
        os.path.join(ROOT, '..', '*.md'),
    ]
    for pattern in patterns:
        for f in glob.glob(pattern, recursive=True):
            try:
                with open(f, encoding='utf-8', errors='ignore') as fh:
                    chars.update(fh.read())
            except OSError:
                pass
    chars.update(chr(c) for c in range(0x20, 0x7F))  # ASCII 可打印区
    chars.update('，。、；：？！「」『』（）《》〈〉【】〔〕…—～·￥％℃×÷±≤≥©®™↑↓←→√†‡§¶')
    chars.update('\n\r\t')
    return ''.join(sorted(chars))


def main() -> int:
    if not os.path.exists(SRC_FONT):
        print(f'未找到全量字体 {SRC_FONT}', file=sys.stderr)
        return 1
    text = collect_chars()
    text_file = '/tmp/font-chars-subset.txt'
    with open(text_file, 'w', encoding='utf-8') as fh:
        fh.write(text)
    out = SRC_FONT + '.subset'
    subprocess.run([
        'pyftsubset', SRC_FONT,
        '--text-file=' + text_file,
        '--flavor=woff2',
        '--output-file=' + out,
        '--layout-features=*',
    ], check=True)
    os.replace(out, SRC_FONT)
    print(f'完成：{len(text)} 字符 -> {SRC_FONT}')
    return 0


if __name__ == '__main__':
    sys.exit(main())
