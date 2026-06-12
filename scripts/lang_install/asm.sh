#!/usr/bin/env bash
set -euo pipefail
apt-get update --allow-releaseinfo-change
# x86 machine code can't be linked or run natively on a non-x86 host (e.g. arm64).
# nasm cross-assembles elf32/elf64 on any host; the prefixed binutils provide x86
# cross linkers; qemu-user-static runs the resulting x86 binary via emulation.
# On an x86-64 host these still work (near-native), so the language is portable.
apt-get install -y --no-install-recommends \
    nasm \
    binutils-x86-64-linux-gnu \
    binutils-i686-linux-gnu \
    qemu-user-static
nasm --version
x86_64-linux-gnu-ld --version | head -n1
i686-linux-gnu-ld --version | head -n1

# A build phase is a single command, but x86 assembly needs two steps (assemble,
# then link). This wrapper does both and picks 32- vs 64-bit from the source. It
# also answers --version, which the readiness probe runs against build.cmd.
cat > /usr/local/bin/nasm-build <<'WRAPPER'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "--version" ] || [ "${1:-}" = "-v" ]; then
    exec nasm --version
fi
src="$1"
out="$2"
obj="${out}.o"
# 64-bit when the source (comments stripped) uses BITS 64, the syscall
# instruction, or a 64-bit register; otherwise assemble as 32-bit i386.
if sed 's/;.*//' "$src" | grep -qiE '(^|[^[:alnum:]_])(bits[[:space:]]+64|syscall|r(ax|bx|cx|dx|si|di|bp|sp|8|9|1[0-5]))([^[:alnum:]_]|$)'; then
    nasm -f elf64 -o "$obj" "$src"
    x86_64-linux-gnu-ld -o "$out" "$obj"
else
    nasm -f elf32 -o "$obj" "$src"
    i686-linux-gnu-ld -o "$out" "$obj"
fi
WRAPPER
chmod 0755 /usr/local/bin/nasm-build

# Run wrapper: select the matching qemu by the artifact's ELF class byte
# (offset 4: 1 = ELFCLASS32, 2 = ELFCLASS64).
cat > /usr/local/bin/nasm-run <<'WRAPPER'
#!/usr/bin/env bash
set -euo pipefail
bin="$1"
cls=$(od -An -tu1 -j4 -N1 "$bin" | tr -d '[:space:]')
if [ "$cls" = "1" ]; then
    exec qemu-i386-static "$bin"
else
    exec qemu-x86_64-static "$bin"
fi
WRAPPER
chmod 0755 /usr/local/bin/nasm-run

/usr/local/bin/nasm-build --version
