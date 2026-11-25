#!/bin/sh
# Pinned upstream v1.5.0 ARM64; no network and no untrusted execution.
set -eu
[ "$#" -eq 2 ] || exit 2
[ "$(sha256sum "$1" | cut -d ' ' -f1)" = b5faaf26d1179fb13f114b0afeae35fd8fbeae52d695027a13917f4876a0f155 ]
[ "$(sha256sum "$2" | cut -d ' ' -f1)" = 27cf7baf54b3859219c31bb1022cd49a7f524c5adf9a3841d2d65cfffbd47f2c ]
umask 077
tmp=$(mktemp -d /tmp/tf-verify.XXXXXX)
trap 'rm -rf "$tmp"' EXIT
trap 'exit 1' HUP INT TERM
(printf '\060\052\060\005\006\003\053\145\160\003\041\000'; printf '%s\n' 'bP7NiKgkGYPrGf03hYHGEhM/nW1ZiXyfx6ewvLVXkLw=' | openssl base64 -d) > "$tmp/pub.der"
openssl base64 -d -in "$2" -out "$tmp/sig"
printf 'trackferry\n1.5.0\nlinux/arm64\n%s\n' b5faaf26d1179fb13f114b0afeae35fd8fbeae52d695027a13917f4876a0f155 > "$tmp/msg"
openssl pkeyutl -verify -pubin -keyform DER -inkey "$tmp/pub.der" -rawin -in "$tmp/msg" -sigfile "$tmp/sig"
