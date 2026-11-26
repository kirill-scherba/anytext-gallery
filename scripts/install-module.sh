#!/usr/bin/env bash
# install-module.sh — build the gallery module and put it where it is used.
#
#   ./scripts/install-module.sh build                 build the static binary into dist/
#   ./scripts/install-module.sh install               install here, restart the module, check it answers
#   ./scripts/install-module.sh install --remote HOST install on another machine through the ssh-agent
#   ./scripts/install-module.sh publish               upload the binary and its sha256 to the bucket
#   ./scripts/install-module.sh all                   build + install (what is used most of the time)
#
# Why a script and not a module catalogue: the core cannot yet install a module from an address, so
# publishing a module means building it, copying the binary and writing the installation's registry
# entry. The script does exactly that and hides nothing — every command it runs is visible in its
# output.
#
# Settings come from the registry of this installation: when the gallery is already installed, its
# collections and address are reused and only the binary is replaced. Otherwise the defaults are used;
# they can be overridden with BUCKET, ARTICLES, PUBLIC_BASE, ADDR and INSTALL.
set -euo pipefail
cd "$(dirname "$0")/.."

BIN_NAME=anytext-gallery
REMOTE=""
ACTION="${1:-all}"
shift || true
while [ $# -gt 0 ]; do
  case "$1" in
    --remote) REMOTE="${2:?--remote needs user@host}"; shift 2 ;;
    --remote=*) REMOTE="${1#*=}"; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

VERSION="$(grep -oP 'const version = "\K[^"]+' main.go)"
DIST="dist"
OUT="$DIST/$BIN_NAME-linux-amd64"

# Defaults: the same values as in main.go.
BUCKET="${BUCKET:-галерея}"
ARTICLES="${ARTICLES:-мемуары}"
PUBLIC_BASE="${PUBLIC_BASE:-https://bmat.uk/i/pub}"
ADDR="${ADDR:-127.0.0.1:7860}"
INSTALL="${INSTALL:-${ANYTEXT_SELF:-http://127.0.0.1:7787}}"
REGISTRY="${REGISTRY:-$HOME/.config/teonet/anytext/modules/modules.json}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

log() { printf '\033[1m== %s\033[0m\n' "$*"; }

# reuseRegistry takes the collections and the address from this installation's entry: updating a module
# must not change the way it is configured.
reuseRegistry() {
  [ -f "$REGISTRY" ] || return 0
  local line
  line=$(python3 - "$REGISTRY" <<'PY'
import json, sys
try:
    mods = json.load(open(sys.argv[1]))
except Exception:
    raise SystemExit(0)
for m in mods:
    if m.get("id") == "gallery":
        cmd = m.get("command") or []
        # the flag values; the binary is replaced anyway
        flags = {"-bucket": "BUCKET", "-articles": "ARTICLES", "-public-base": "PUBLIC_BASE", "-addr": "ADDR"}
        out = []
        i = 0
        while i < len(cmd) - 1:
            if cmd[i] in flags:
                out.append("%s=%s" % (flags[cmd[i]], cmd[i + 1]))
                i += 2
                continue
            i += 1
        print("\n".join(out))
        break
PY
)
  while IFS='=' read -r key val; do
    [ -n "${key:-}" ] || continue
    case "$key" in
      BUCKET) BUCKET="$val" ;;
      ARTICLES) ARTICLES="$val" ;;
      PUBLIC_BASE) PUBLIC_BASE="$val" ;;
      ADDR) ADDR="$val" ;;
    esac
  done <<< "$line"
}

build() {
  log "building $BIN_NAME $VERSION"
  mkdir -p "$DIST"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags "-s -w" -o "$OUT" .
  ( cd "$DIST" && sha256sum "$BIN_NAME-linux-amd64" | tee "$BIN_NAME-linux-amd64.sha256" ) >/dev/null
  ls -lh "$OUT" | awk '{print "   " $9 " " $5}'
  echo "   sha256 $(cut -d' ' -f1 "$OUT.sha256")"
}

# registryJSON builds the body of modules.install. JSON is assembled by python: collection names in
# Russian and quotes in paths must not depend on how careful the shell is.
registryJSON() {
  local bin_path="$1"
  python3 - "$bin_path" "$VERSION" "$ADDR" "$BUCKET" "$ARTICLES" "$PUBLIC_BASE" <<'PY'
import json, sys
bin_path, version, addr, bucket, articles, public_base = sys.argv[1:7]
cmd = [bin_path, "-addr", addr, "-bucket", bucket]
if public_base:
    cmd += ["-public-base", public_base]
if articles:
    cmd += ["-articles", articles]
else:
    cmd += ["-articles", ""]
print(json.dumps({
    "command": "modules.install",
    "args": {
        "id": "gallery",
        "name": "Галерея",
        "names": {"ru": "Галерея", "en": "Gallery"},
        "version": version,
        "addr": addr,
        "command": cmd,
        "auto_start": True,
        "note": "картинки и статьи",
    },
}, ensure_ascii=False))
PY
}

calling() {
  curl -s -X POST "$INSTALL/api/command" -H 'Content-Type: application/json' -d "$1"
}

installLocal() {
  log "installing into $INSTALL"
  install -D -m 0755 "$OUT" "$BIN_DIR/$BIN_NAME"
  echo "   binary: $BIN_DIR/$BIN_NAME"
  calling "$(registryJSON "$BIN_DIR/$BIN_NAME")" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("   registry:", d.get("result", d))'
  calling '{"command":"modules.stop","args":{"id":"gallery"}}' >/dev/null 2>&1 || true
  sleep 1
  calling '{"command":"modules.start","args":{"id":"gallery"}}' >/dev/null 2>&1 || true
  verify
}

# verify asks the module what version it is: a binary built after a change but without a version bump
# looks old while the feature is already inside, and only the module itself can tell.
verify() {
  log "checking"
  local out
  for _ in $(seq 1 20); do
    if out=$(curl -s -m 3 "$INSTALL/api/module/gallery/api/status"); then
      python3 - "$VERSION" "$out" <<'PY' && return 0
import json, sys
want, raw = sys.argv[1], sys.argv[2]
try:
    d = json.loads(raw)
except Exception:
    raise SystemExit(1)
if not d.get("version"):
    raise SystemExit(1)
print("   version %s, pictures %s, ready %s" % (d.get("version"), d.get("pictures"), d.get("ready")))
if d.get("version") != want:
    print("   WARNING: the installation answers with another version")
    raise SystemExit(1)
PY
    fi
    sleep 1
  done
  echo "   the module did not answer — see $HOME/.config/teonet/anytext/modules/gallery.log" >&2
  return 1
}

# sshSocket finds an agent socket that holds a key: there may be no key files in ~/.ssh at all, while
# access exists (memory/technical/ssh/agent-socket-rule).
sshSocket() {
  local s
  for s in "$HOME"/.ssh/agent/*; do
    [ -S "$s" ] || continue
    if SSH_AUTH_SOCK="$s" ssh-add -l >/dev/null 2>&1; then
      echo "$s"
      return 0
    fi
  done
  return 1
}

installRemote() {
  log "installing into $REMOTE"
  local sock remote_bin body
  sock=$(sshSocket) || { echo "no ssh agent with a key (see memory/technical/ssh/agent-socket-rule)" >&2; exit 1; }
  # the home directory of the remote user may differ: ask the machine itself
  remote_bin=$(SSH_AUTH_SOCK="$sock" ssh -o BatchMode=yes "$REMOTE" 'echo "$HOME/.local/bin/anytext-gallery"')
  SSH_AUTH_SOCK="$sock" ssh -o BatchMode=yes "$REMOTE" 'mkdir -p "$(dirname "$HOME/.local/bin/anytext-gallery")"'
  SSH_AUTH_SOCK="$sock" scp -q "$OUT" "$REMOTE:$remote_bin.new"
  SSH_AUTH_SOCK="$sock" ssh -o BatchMode=yes "$REMOTE" "install -m 0755 '$remote_bin.new' '$remote_bin' && rm -f '$remote_bin.new' && echo '   binary: $remote_bin'"
  # the installation writes its own registry and restarts the module: use its /api/command there
  body=$(registryJSON "$remote_bin")
  SSH_AUTH_SOCK="$sock" ssh -o BatchMode=yes "$REMOTE" "curl -s -X POST http://127.0.0.1:7787/api/command -H 'Content-Type: application/json' -d $(printf '%q' "$body") >/dev/null && \
     curl -s -X POST http://127.0.0.1:7787/api/command -H 'Content-Type: application/json' -d '{\"command\":\"modules.stop\",\"args\":{\"id\":\"gallery\"}}' >/dev/null; sleep 1; \
     curl -s -X POST http://127.0.0.1:7787/api/command -H 'Content-Type: application/json' -d '{\"command\":\"modules.start\",\"args\":{\"id\":\"gallery\"}}' >/dev/null; sleep 2; \
     curl -s http://127.0.0.1:7787/api/module/gallery/api/status"
  echo
}

publish() {
  log "publishing into the bucket"
  local bucket="gs://cooksy-ea86d-anytext-releases/modules/gallery/$VERSION"
  local gsutil="${GSUTIL:-$HOME/google-cloud-sdk/bin/gsutil}"
  "$gsutil" -q cp "$OUT" "$bucket/$BIN_NAME-linux-amd64"
  "$gsutil" -q cp "$OUT.sha256" "$bucket/$BIN_NAME-linux-amd64.sha256"
  "$gsutil" -q setmeta -h "Cache-Control:no-store" "$bucket/$BIN_NAME-linux-amd64" >/dev/null
  echo "   $bucket/$BIN_NAME-linux-amd64"
  echo "   sha256 $(cut -d' ' -f1 "$OUT.sha256")"
  echo "   address: https://storage.googleapis.com/cooksy-ea86d-anytext-releases/modules/gallery/$VERSION/$BIN_NAME-linux-amd64"
}

reuseRegistry
echo "version $VERSION · collections: pictures «$BUCKET», articles «${ARTICLES:-none}» · address $ADDR"

case "$ACTION" in
  build) build ;;
  install)
    [ -f "$OUT" ] || build
    if [ -n "$REMOTE" ]; then installRemote; else installLocal; fi
    ;;
  publish) build; publish ;;
  all)
    build
    if [ -n "$REMOTE" ]; then installRemote; else installLocal; fi
    ;;
  *) echo "usage: $0 {build|install|publish|all} [--remote user@host]" >&2; exit 2 ;;
esac
