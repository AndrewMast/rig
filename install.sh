#!/usr/bin/env bash
# rig installer. Downloads a release over HTTPS (no gh dependency), verifies it
# through a tiered ladder, installs the binary, and offers shell integration.
#
#   curl -fsSL https://raw.githubusercontent.com/AndrewMast/rig/master/install.sh | bash
#
# Environment / flags:
#   INSTALL_DIR=~/.local/bin       install location (default)
#   VERSION=v0.1.0                 install a specific tag (default: latest)
#   --without-attestation          skip attestation + its prompt (checksum only)
#   --require-attestation          hard-fail without verified provenance
set -euo pipefail

OWNER="AndrewMast"
REPO="rig"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
WITHOUT_ATTESTATION="${RIG_WITHOUT_ATTESTATION:-}"
REQUIRE_ATTESTATION="${RIG_REQUIRE_ATTESTATION:-}"

for arg in "$@"; do
  case "$arg" in
    --without-attestation) WITHOUT_ATTESTATION=1 ;;
    --require-attestation) REQUIRE_ATTESTATION=1 ;;
    *) echo "rig install: unknown flag $arg" >&2; exit 2 ;;
  esac
done

err() { echo "rig install: $*" >&2; exit 1; }
note() { echo "==> $*"; }

# --- platform -------------------------------------------------------------
os="$(uname -s)"
[ "$os" = "Darwin" ] || err "only macOS is supported (got $os)"
case "$(uname -m)" in
  arm64) arch="arm64" ;;
  x86_64) arch="amd64" ;;
  *) err "unsupported architecture $(uname -m)" ;;
esac

# --- version --------------------------------------------------------------
version="${VERSION:-}"
if [ -z "$version" ]; then
  note "resolving latest release"
  version="$(curl -fsSL "https://api.github.com/repos/$OWNER/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  [ -n "$version" ] || err "could not resolve latest version"
fi
ver_no_v="${version#v}"
base="https://github.com/$OWNER/$REPO/releases/download/$version"
archive="rig_${ver_no_v}_darwin_${arch}.tar.gz"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"

note "downloading $archive ($version)"
curl -fsSL -o "$archive" "$base/$archive" || err "download failed: $base/$archive"
curl -fsSL -o checksums.txt "$base/checksums.txt" || err "could not download checksums.txt"

# --- ladder step 1: checksum (always) -------------------------------------
note "verifying SHA256 checksum"
want="$(grep " $archive\$" checksums.txt | awk '{print $1}')"
[ -n "$want" ] || err "no checksum listed for $archive"
got="$(shasum -a 256 "$archive" | awk '{print $1}')"
[ "$want" = "$got" ] || err "checksum mismatch (want $want, got $got)"

# --- ladder steps 2-5: attestation ----------------------------------------
verify_attestation() {
  curl -fsSL -o rig.attestation.jsonl "$base/rig.attestation.jsonl" 2>/dev/null || return 1
  gh attestation verify "$archive" --bundle rig.attestation.jsonl --owner "$OWNER" >/dev/null 2>&1
}

if command -v gh >/dev/null 2>&1; then
  note "verifying build provenance with gh (offline)"
  if verify_attestation; then
    note "attestation verified"
  elif [ -n "$REQUIRE_ATTESTATION" ]; then
    err "attestation verification failed and --require-attestation is set"
  else
    note "attestation could not be verified; continuing on checksum only"
  fi
elif [ -n "$REQUIRE_ATTESTATION" ]; then
  err "--require-attestation set but gh is not installed"
elif [ -n "$WITHOUT_ATTESTATION" ]; then
  note "skipping attestation (--without-attestation)"
else
  if [ -t 0 ]; then
    printf "gh not found; install with checksum verification only? [y/N] "
    read -r ans
    case "$ans" in y|Y|yes) ;; *) err "aborted" ;; esac
  else
    note "gh not found, non-interactive: continuing on checksum only"
  fi
fi

# --- install --------------------------------------------------------------
note "installing to $INSTALL_DIR"
tar -xzf "$archive"
mkdir -p "$INSTALL_DIR"
install -m 0755 rig "$INSTALL_DIR/rig"
note "installed rig $version to $INSTALL_DIR/rig"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) note "note: $INSTALL_DIR is not on your PATH" ;;
esac

# --- shell integration (auto-offer) ---------------------------------------
rig_sh="$HOME/.config/rig/rig.sh"
zshrc="$HOME/.zshrc"
source_line="[ -f \"\$HOME/.config/rig/rig.sh\" ] && source \"\$HOME/.config/rig/rig.sh\""

if [ ! -f "$zshrc" ] || ! grep -qF "/.config/rig/rig.sh" "$zshrc"; then
  do_it=""
  if [ -t 0 ]; then
    printf "Enable shell integration (rig cd + completions) in ~/.zshrc? [Y/n] "
    read -r ans
    case "$ans" in n|N|no) ;; *) do_it=1 ;; esac
  fi
  if [ -n "$do_it" ]; then
    mkdir -p "$(dirname "$rig_sh")"
    printf 'command -v rig >/dev/null 2>&1 && eval "$(rig shell-init zsh)"\n' > "$rig_sh"
    printf '\n# rig shell integration\n%s\n' "$source_line" >> "$zshrc"
    note "added shell integration to $zshrc — restart your shell or 'source $zshrc'"
  fi
fi

note "done"
