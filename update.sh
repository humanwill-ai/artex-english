#!/usr/bin/env bash
# ARTEX 업데이트 스크립트: ① Docker 업데이트(새 이미지를 받아 재구성)  ② 로컬 컴파일 업데이트(바이너리 재빌드)
# install.sh 와 짝을 이룹니다. install 은 최초 설치를, update 는 새 버전으로의 업그레이드를 담당합니다.
# DB 마이그레이션은 직접 실행할 필요가 없습니다. artex 는 기동할 때마다 schema.sql 을 멱등하게 다시 돌리므로(ADD COLUMN/CREATE
# INDEX IF NOT EXISTS 포함) "재시작이 곧 마이그레이션"입니다. 데이터(pgdata 볼륨, ./data, ./skills)는 영향을 받지 않습니다.
set -euo pipefail
cd "$(cd "$(dirname "$0")" && pwd)"

info(){ printf '\033[36m[*]\033[0m %s\n' "$*"; }
ok(){   printf '\033[32m[+]\033[0m %s\n' "$*"; }
warn(){ printf '\033[33m[!]\033[0m %s\n' "$*"; }
die(){  printf '\033[31m[x]\033[0m %s\n' "$*" >&2; exit 1; }
ask(){  local p="$1" d="${2:-}" a; read -rp "$p${d:+ [$d]}: " a; echo "${a:-$d}"; }

# ── 선택: 저장소를 최신 코드로 동기화합니다(compose·스크립트·로컬 컴파일 소스가 모두 이걸로 갱신됩니다) ───────
sync_repo(){
  [ -d .git ] && command -v git >/dev/null 2>&1 || { warn "Not a Git checkout; skipping git pull"; return; }
  [ "$(ask 'Fetch the latest code (git pull --ff-only)? (y/n)' y)" = y ] || return
  if ! git pull --ff-only; then
    warn "git pull could not fast-forward (local changes or diverged branches). Resolve this manually and retry. Continuing with the current code."
  fi
}

# ── ① Docker 업데이트 ───────────────────────────────
update_docker(){
  die "No HumanWill English image is published yet; use the local source-build option"
}

# ── ② 로컬 컴파일 업데이트 ──────────────────────────────
update_local(){
  command -v go >/dev/null 2>&1 || die "Go not found (>=1.26 required): https://go.dev/dl/"
  [ -f config.json ] || warn "config.json not found. For an initial deployment, use ./install.sh."
  ok "Go: $(go version)"

  if command -v npm >/dev/null 2>&1; then
    info "Rebuilding static frontend assets..."
    ( cd web && npm ci && npm run build:static )
    rm -rf server/webui/dist && cp -r web/out server/webui/dist
    info "Recompiling the single binary with the embedded frontend..."
    CGO_ENABLED=0 go build -tags embedui -trimpath -o artex ./cmd/artex
  else
    warn "npm not found. Building the backend without the frontend (run the frontend separately with npm run dev)."
    CGO_ENABLED=0 go build -o artex ./cmd/artex
  fi
  ok "Build complete -> ./artex"
  warn "Restart the running artex process to apply changes (schema migration runs automatically on restart)."
}

warn "No HumanWill English image is published yet. Choose option 2 to build this English checkout locally."
echo "=============================="
echo "  ARTEX update"
echo "  1) Prebuilt Docker update (unavailable)"
echo "  2) Local update (recompile with Go)"
echo "=============================="
case "$(ask 'Selection' 1)" in
  1) update_docker ;;
  2) sync_repo; update_local ;;
  *) die "Invalid selection" ;;
esac
