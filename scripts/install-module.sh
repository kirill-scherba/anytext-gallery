#!/usr/bin/env bash
# install-module.sh — build the gallery module and put it where it is used.
#
#   ./scripts/install-module.sh build                 собрать статический бинарь в dist/
#   ./scripts/install-module.sh install               поставить на этой машине и перезапустить модуль
#   ./scripts/install-module.sh install --remote HOST поставить на другой машине через ssh-agent
#   ./scripts/install-module.sh publish               положить бинарь в бакет вместе с sha256
#   ./scripts/install-module.sh all                   build + install (то, что делается чаще всего)
#
# Почему это скрипт, а не каталог модулей: пока в ядре нет установки по адресу, публикация модуля —
# это сборка, копия бинаря и запись в реестр установки. Скрипт делает ровно это и ничего не держит в
# секрете: каждая команда, которую он выполняет, видна в выводе.
#
# Настройки берутся из реестра модулей этой установки: если галерея уже стоит, её коллекции и адрес
# переиспользуются, меняется только бинарь. Иначе — значения по умолчанию (их можно переопределить
# переменными окружения BUCKET, ARTICLES, PUBLIC_BASE, ADDR и INSTALL).
set -euo pipefail
cd "$(dirname "$0")/.."

BIN_NAME=anytext-gallery
REMOTE=""
ACTION="${1:-all}"
shift || true
while [ $# -gt 0 ]; do
  case "$1" in
    --remote) REMOTE="${2:?--remote требует user@host}"; shift 2 ;;
    --remote=*) REMOTE="${1#*=}"; shift ;;
    *) echo "неизвестный аргумент: $1" >&2; exit 2 ;;
  esac
done

VERSION="$(grep -oP 'const version = "\K[^"]+' main.go)"
DIST="dist"
OUT="$DIST/$BIN_NAME-linux-amd64"
SRC_ROOT="$(pwd)"

# Значения по умолчанию: те же, что в main.go.
BUCKET="${BUCKET:-галерея}"
ARTICLES="${ARTICLES:-мемуары}"
PUBLIC_BASE="${PUBLIC_BASE:-https://bmat.uk/i/pub}"
ADDR="${ADDR:-127.0.0.1:7860}"
INSTALL="${INSTALL:-${ANYTEXT_SELF:-http://127.0.0.1:7787}}"
REGISTRY="${REGISTRY:-$HOME/.config/teonet/anytext/modules/modules.json}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

log() { printf '\033[1m== %s\033[0m\n' "$*"; }

# reusesRegistry берёт коллекции и адрес из записи этой установки: обновление модуля не должно
# менять его настройки.
reusesRegistry() {
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
        # значения флагов; адрес берём из записи, бинарь всё равно подменим
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
  log "сборка $BIN_NAME $VERSION"
  mkdir -p "$DIST"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags "-s -w" -o "$OUT" .
  ( cd "$DIST" && sha256sum "$BIN_NAME-linux-amd64" | tee "$BIN_NAME-linux-amd64.sha256" ) >/dev/null
  ls -lh "$OUT" | awk '{print "   " $9 " " $5}'
  echo "   sha256 $(cut -d' ' -f1 "$OUT.sha256")"
}

# registryJSON собирает тело modules.install. JSON собирается питоном: русские имена коллекций и
# кавычки в путях не должны зависеть от аккуратности shell.
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
  log "установка на $INSTALL"
  install -D -m 0755 "$OUT" "$BIN_DIR/$BIN_NAME"
  echo "   бинарь: $BIN_DIR/$BIN_NAME"
  calling "$(registryJSON "$BIN_DIR/$BIN_NAME")" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("   реестр:", d.get("result", d))'
  calling '{"command":"modules.stop","args":{"id":"gallery"}}' >/dev/null 2>&1 || true
  sleep 1
  calling '{"command":"modules.start","args":{"id":"gallery"}}' >/dev/null 2>&1 || true
  verify
}

verify() {
  log "проверка"
  for _ in $(seq 1 20); do
    if out=$(curl -s -m 3 "$INSTALL/api/module/gallery/api/status"); then
      # status отвечает JSON: версия, коллекция, число картинок
      python3 - "$VERSION" <<PY
import json, sys
try:
    d = json.loads("""$out""")
except Exception:
    sys.exit(1)
if d.get("version"):
    print("   версия %s, картинок %s, готовность %s" % (d.get("version"), d.get("pictures"), d.get("ready")))
    if d.get("version") != sys.argv[1]:
        print("   ВНИМАНИЕ: установка сообщает другую версию")
        sys.exit(1)
    sys.exit(0)
sys.exit(1)
PY
      return 0
    fi
    sleep 1
  done
  echo "   модуль не ответил — смотрите $HOME/.config/teonet/anytext/modules/gallery.log" >&2
  return 1
}

# sshSocket находит сокет агента с ключом: ключей в ~/.ssh может не быть вовсе, а доступ есть.
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
  log "установка на $REMOTE"
  local sock
  sock=$(sshSocket) || { echo "нет ssh-агента с ключом (см. memory/technical/ssh/agent-socket-rule)" >&2; exit 1; }
  local remote_bin="/home/$USER/.local/bin/$BIN_NAME"
  # путь у удалённого пользователя может отличаться: спрашиваем его у самой машины
  remote_bin=$(SSH_AUTH_SOCK="$sock" ssh -o BatchMode=yes "$REMOTE" 'echo "$HOME/.local/bin/anytext-gallery"')
  SSH_AUTH_SOCK="$sock" ssh -o BatchMode=yes "$REMOTE" 'mkdir -p "$(dirname "$HOME/.local/bin/anytext-gallery")"'
  SSH_AUTH_SOCK="$sock" scp -q "$OUT" "$REMOTE:$remote_bin.new"
  SSH_AUTH_SOCK="$sock" ssh -o BatchMode=yes "$REMOTE" "install -m 0755 '$remote_bin.new' '$remote_bin' && rm -f '$remote_bin.new' && echo '   бинарь: $remote_bin'"
  # установка сама пишет реестр и перезапускает модуль: делаем это через её же /api/command
  SSH_AUTH_SOCK="$sock" ssh -o BatchMode=yes "$REMOTE" \
    "curl -s -X POST http://127.0.0.1:7787/api/command -H 'Content-Type: application/json' -d $(printf '%q' "$(registryJSON "$remote_bin")") >/dev/null && \
     curl -s -X POST http://127.0.0.1:7787/api/command -H 'Content-Type: application/json' -d '{\"command\":\"modules.stop\",\"args\":{\"id\":\"gallery\"}}' >/dev/null; sleep 1; \
     curl -s -X POST http://127.0.0.1:7787/api/command -H 'Content-Type: application/json' -d '{\"command\":\"modules.start\",\"args\":{\"id\":\"gallery\"}}' >/dev/null; sleep 2; \
     curl -s http://127.0.0.1:7787/api/module/gallery/api/status"
  echo
}

publish() {
  log "публикация в бакет"
  local bucket="gs://cooksy-ea86d-anytext-releases/modules/gallery/$VERSION"
  local gsutil="${GSUTIL:-$HOME/google-cloud-sdk/bin/gsutil}"
  "$gsutil" -q cp "$OUT" "$bucket/$BIN_NAME-linux-amd64"
  "$gsutil" -q cp "$OUT.sha256" "$bucket/$BIN_NAME-linux-amd64.sha256"
  "$gsutil" -q setmeta -h "Cache-Control:no-store" "$bucket/$BIN_NAME-linux-amd64" >/dev/null
  echo "   $bucket/$BIN_NAME-linux-amd64"
  echo "   sha256 $(cut -d' ' -f1 "$OUT.sha256")"
  echo "   адрес: https://storage.googleapis.com/cooksy-ea86d-anytext-releases/modules/gallery/$VERSION/$BIN_NAME-linux-amd64"
}

reusesRegistry
echo "версия $VERSION · коллекции: картинки «$BUCKET», статьи «${ARTICLES:-нет}» · адрес $ADDR"

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
  *) echo "использование: $0 {build|install|publish|all} [--remote user@host]" >&2; exit 2 ;;
esac
