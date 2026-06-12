#!/usr/bin/env bash
set -euo pipefail
# Assembly x86 (32-bit, NASM). Targets x86-64 Linux hosts: nasm assembles to
# 32-bit i386, ld links it, and the binary runs natively (32-bit ELF executes via
# the kernel's IA32 emulation). Not intended to run on non-x86 hosts.
apt-get update --allow-releaseinfo-change
apt-get install -y --no-install-recommends nasm binutils
nasm --version
ld --version | head -n1

# A build phase is a single command, but assembly needs two steps (assemble with
# nasm, then link with ld). This wrapper does both. It also answers --version,
# which the readiness probe runs against build.cmd.
cat > /usr/local/bin/nasm-build <<'WRAPPER'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "--version" ] || [ "${1:-}" = "-v" ]; then
    exec nasm --version
fi
src="$1"
out="$2"
obj="${out}.o"
nasm -f elf32 -o "$obj" "$src"
ld -m elf_i386 -o "$out" "$obj"
WRAPPER
chmod 0755 /usr/local/bin/nasm-build
/usr/local/bin/nasm-build --version
