const script = `#!/bin/sh
set -eu

API=""
TOKEN=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --api)
      API="$2"
      shift 2
      ;;
    --token)
      TOKEN="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

if [ -z "$API" ] || [ -z "$TOKEN" ]; then
  echo "Usage: install.sh --api <url> --token <token>" >&2
  exit 2
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported CPU architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported operating system: $OS" >&2; exit 1 ;;
esac

APP_DIR="\${XDG_DATA_HOME:-$HOME/.local/share}/vibeengine"
BIN_DIR="$APP_DIR/bin"
SHIM_DIR="$HOME/.local/bin"
ENGINE="$BIN_DIR/vibeengine"
ARTIFACT="vibeengine-$OS-$ARCH"

mkdir -p "$BIN_DIR" "$SHIM_DIR"

download_engine() {
  tmp="$ENGINE.tmp"
  sig="$tmp.sig"
  checksum="$tmp.sha256"
  pub="$tmp.pub"
  verify_tmp="$tmp.verify"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$API/releases/$ARTIFACT" -o "$tmp"
    curl -fsSL "$API/releases/$ARTIFACT.sig" -o "$sig"
    curl -fsSL "$API/releases/$ARTIFACT.sha256" -o "$checksum"
    curl -fsSL "$API/releases/release-signing-public.pem" -o "$pub"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$tmp" "$API/releases/$ARTIFACT"
    wget -qO "$sig" "$API/releases/$ARTIFACT.sig"
    wget -qO "$checksum" "$API/releases/$ARTIFACT.sha256"
    wget -qO "$pub" "$API/releases/release-signing-public.pem"
  else
    echo "Install needs curl or wget to download Vibeengine." >&2
    return 1
  fi
  if ! command -v openssl >/dev/null 2>&1; then
    echo "Install needs openssl to verify Vibeengine release signatures." >&2
    return 1
  fi
  expected="$(awk '{print $1}' "$checksum")"
  actual="$(openssl dgst -sha256 -r "$tmp" | awk '{print $1}')"
  if [ "$expected" != "$actual" ]; then
    echo "Vibeengine release checksum verification failed." >&2
    return 1
  fi
  if ! openssl dgst -sha256 -verify "$pub" -signature "$sig" "$tmp" > "$verify_tmp" 2>&1; then
    cat "$verify_tmp" >&2 || true
    echo "Vibeengine release signature verification failed." >&2
    return 1
  fi
  chmod 700 "$tmp"
  mv "$tmp" "$ENGINE"
  rm -f "$sig" "$checksum" "$pub" "$verify_tmp"
}

if ! download_engine; then
  rm -f "$ENGINE.tmp" "$ENGINE.tmp.sig" "$ENGINE.tmp.sha256" "$ENGINE.tmp.pub" "$ENGINE.tmp.verify"
  if command -v vibeengine >/dev/null 2>&1; then
    ENGINE="$(command -v vibeengine)"
  else
    echo "No verified Vibeengine release artifact is available for $OS/$ARCH yet." >&2
    exit 1
  fi
fi

if [ "$ENGINE" != "$(command -v vibeengine 2>/dev/null || true)" ]; then
  ln -sf "$ENGINE" "$SHIM_DIR/vibeengine" 2>/dev/null || cp "$ENGINE" "$SHIM_DIR/vibeengine"
fi

"$ENGINE" bootstrap --api "$API" --token "$TOKEN" --repo "$(pwd)" --adapter codex --adapter claude-code
"$ENGINE" status

case ":$PATH:" in
  *":$SHIM_DIR:"*) ;;
  *) echo "Add $SHIM_DIR to PATH to run vibeengine from any shell." ;;
esac
`;

export function GET() {
  return new Response(script, {
    headers: {
      "content-type": "text/x-shellscript; charset=utf-8",
      "cache-control": "no-store",
    },
  });
}
