#!/usr/bin/env bash
# ARTEX 설치 스크립트: ① 전부 Docker  ② 로컬 컴파일 실행
set -euo pipefail
cd "$(cd "$(dirname "$0")" && pwd)"

info(){ printf '\033[36m[*]\033[0m %s\n' "$*"; }
ok(){   printf '\033[32m[+]\033[0m %s\n' "$*"; }
warn(){ printf '\033[33m[!]\033[0m %s\n' "$*"; }
die(){  printf '\033[31m[x]\033[0m %s\n' "$*" >&2; exit 1; }
ask(){  local p="$1" d="${2:-}" a; read -rp "$p${d:+ [$d]}: " a; echo "${a:-$d}"; }
rand(){ head -c 18 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 24; }

# ── docker 환경 감지 / 자동 설치 ──────────────────
ensure_docker(){
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    ok "docker and docker compose are available"; return
  fi
  warn "docker / docker compose not found"
  case "$(uname -s)" in
    Linux)
      if [ "$(ask 'Install Docker automatically? (y/n)' y)" = y ]; then
        curl -fsSL https://get.docker.com | sh
        sudo usermod -aG docker "$USER" || true
        ok "Docker installed (log in again for group changes to allow use without sudo)"
      else
        die "Install docker manually, then run this script again"
      fi ;;
    Darwin) die "On macOS, install Docker Desktop: https://www.docker.com/products/docker-desktop/" ;;
    *)      die "Install docker manually, then run this script again" ;;
  esac
}

# ── ① 전부 Docker ──────────────────────────────
install_docker(){
  die "No HumanWill English image is published yet; use the local source-build option"
}

# ── ② 로컬 컴파일 실행 ────────────────────────────
install_local(){
  echo "Database setup:"
  echo "  1) Connect to an existing PostgreSQL database"
  echo "  2) Start PostgreSQL in Docker (requires docker)"
  case "$(ask 'Selection' 1)" in
    2)
      ensure_docker
      local pw; pw="$(ask 'Postgres password (Enter for a random value)' "$(rand)")"
      docker run -d --name artex-pg -p 5432:5432 \
        -e POSTGRES_USER=artex -e POSTGRES_PASSWORD="$pw" -e POSTGRES_DB=artex \
        -v artex-pg:/var/lib/postgresql/data postgres:16-alpine
      DB_HOST=127.0.0.1 DB_PORT=5432 DB_USER=artex DB_PASS="$pw" DB_NAME=artex DB_SSL=disable ;;
    *)
      DB_HOST="$(ask 'Database host' 127.0.0.1)"
      DB_PORT="$(ask 'Port' 5432)"
      DB_USER="$(ask 'Username' artex)"
      DB_PASS="$(ask 'Password' '')"
      DB_NAME="$(ask 'Database name' artex)"
      DB_SSL="$(ask 'sslmode (disable/require)' disable)" ;;
  esac

  # config.json 생성
  cat > config.json <<JSON
{
  "database": {
    "host": "${DB_HOST}",
    "port": ${DB_PORT},
    "user": "${DB_USER}",
    "password": "${DB_PASS}",
    "dbname": "${DB_NAME}",
    "sslmode": "${DB_SSL}"
  }
}
JSON
  ok "config.json created"

  # go 환경 확인
  command -v go >/dev/null 2>&1 || die "Go not found. Install Go (>=1.26) first: https://go.dev/dl/"
  ok "Go: $(go version)"

  # 프런트엔드를 내장하려면 node 로 정적 산출물을 만들어야 합니다
  if command -v npm >/dev/null 2>&1; then
    info "Building static frontend assets..."
    ( cd web && npm ci && npm run build:static )
    rm -rf server/webui/dist && mkdir -p server/webui && cp -r web/out server/webui/dist
    info "Compiling the single binary with the embedded frontend..."
    CGO_ENABLED=0 go build -tags embedui -trimpath -o artex ./cmd/artex
  else
    warn "npm not found: building only the backend without the frontend (run the frontend separately with npm run dev)."
    CGO_ENABLED=0 go build -o artex ./cmd/artex
  fi
  ok "Build complete -> ./artex"

  info "Starting... (Ctrl-C to stop)"
  ./artex
}

warn "No HumanWill English image is published yet. Choose option 2 to build this English checkout locally."
echo "=============================="
echo "  ARTEX installation"
echo "  1) Prebuilt Docker installation (unavailable)"
echo "  2) Local run (compile with Go)"
echo "=============================="
case "$(ask 'Selection' 1)" in
  1) install_docker ;;
  2) install_local ;;
  *) die "Invalid selection" ;;
esac
