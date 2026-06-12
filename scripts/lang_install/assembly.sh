#!/usr/bin/env bash
set -euo pipefail

apt-get install -y --no-install-recommends nasm binutils

cat << 'EOF' > /usr/local/bin/nasm-build
#!/bin/bash
set -e
if [ "$1" = "--version" ]; then
    nasm --version
    exit 0
fi
nasm -f elf64 -o "$1.o" "$1"
ld -o "$2" "$1.o"
EOF
chmod +x /usr/local/bin/nasm-build

nasm --version
