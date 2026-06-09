#!/usr/bin/env bash
set -euo pipefail
# perl-base ships /usr/bin/perl in the Debian base image (Priority: required).
# Installing it explicitly is a near-no-op that documents the dependency.
apt-get install -y --no-install-recommends perl-base
perl -e 'print "perl ok\n"'
