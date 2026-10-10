#!/usr/bin/env bash
# =============================================================================
# ARTEX administrator password reset
#
# The login username is fixed as ARTEX. The password is stored as a bcrypt hash
# under auth.password_hash in the settings table. This script connects to the database,
# uses pgcrypto to generate a bcrypt hash, and writes it with backend-compatible hashing.
#
# Two deployment modes:
#   local (default) -- connect using psql on the host. Connection settings take precedence:
#                      command-line arguments > --dsn/$ARTEX_PG_DSN > config.json database.*
#   docker          -- run psql inside the postgres container using docker compose exec
#                      or docker exec (Compose does not expose port 5432 by default).
#
# Examples:
#   ./reset-password.sh                          # Read local config/environment; prompt for password
#   ./reset-password.sh -p 'NewPass!'             # Specify the new password directly
#   ./reset-password.sh --dsn postgres://u:p@h:5432/artex
#   ./reset-password.sh -H 127.0.0.1 -P 5433 -U autopentest -W pass -d artex
#   ./reset-password.sh -m docker                 # Read POSTGRES_* from .env
#   ./reset-password.sh -m docker -c pg-container --exec docker
#
# The new password is passed to psql through the environment and \getenv, not psql argv,
# and escaped with :'var'. The database password is passed through PGPASSWORD, not psql argv.
# =============================================================================
set -euo pipefail

PASS_KEY="auth.password_hash"
BCRYPT_COST=10

MODE=""            # local | docker(비우면 자동 판정)
DSN=""
HOST="" PORT="" USER="" DBPASS="" DBNAME="" SSLMODE=""
CONFIG=""
CONTAINER=""       # docker 모드의 postgres 서비스/컨테이너명(기본값 postgres)
EXEC_KIND=""       # compose | docker(docker 모드에서 어떤 exec 를 쓸지. 비우면 자동)
NEWPASS=""
ASSUME_YES=0

die() { echo "Error: $*" >&2; exit 1; }
info() { echo "· $*" >&2; }

usage() { sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'; exit 0; }

# ---- 인자 파싱 -------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    -m|--mode)        MODE="${2:-}"; shift 2 ;;
    --dsn)            DSN="${2:-}"; shift 2 ;;
    -H|--host)        HOST="${2:-}"; shift 2 ;;
    -P|--port)        PORT="${2:-}"; shift 2 ;;
    -U|--user)        USER="${2:-}"; shift 2 ;;
    -W|--db-password) DBPASS="${2:-}"; shift 2 ;;
    -d|--dbname)      DBNAME="${2:-}"; shift 2 ;;
    --sslmode)        SSLMODE="${2:-}"; shift 2 ;;
    --config)         CONFIG="${2:-}"; shift 2 ;;
    -c|--container)   CONTAINER="${2:-}"; shift 2 ;;
    --exec)           EXEC_KIND="${2:-}"; shift 2 ;;
    -p|--new-password) NEWPASS="${2:-}"; shift 2 ;;
    -y|--yes)         ASSUME_YES=1; shift ;;
    -h|--help)        usage ;;
    *) die "Unknown argument: $1 (use -h for help)" ;;
  esac
done

# ---- config.json 에서 database.* 읽기(local 모드이고 접속 정보를 명시하지 않았을 때만) -----
# python3 로 파싱하는 것을 우선합니다(견고함). python3 가 없으면 grep 으로 대체합니다(config.json 이 정돈된 분리 필드라서).
read_config_json() {
  local path="$1"
  [[ -f "$path" ]] || return 1
  if command -v python3 >/dev/null 2>&1; then
    python3 - "$path" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1])).get("database", {})
except Exception:
    sys.exit(1)
# dsn 을 직접 주거나 필드를 나눠 줄 수 있습니다
if d.get("dsn"):
    print("DSN\t" + d["dsn"]); sys.exit(0)
for k in ("host","port","user","password","dbname","sslmode"):
    if d.get(k) is not None:
        print(k.upper() + "\t" + str(d[k]))
PY
  else
    # 아주 단순한 후순위: 키마다 grep(값은 문자열 또는 숫자)
    local k
    for k in host port user password dbname sslmode; do
      local v
      v=$(grep -oE "\"$k\"[[:space:]]*:[[:space:]]*(\"[^\"]*\"|[0-9]+)" "$path" 2>/dev/null \
            | head -1 | sed -E "s/.*:[[:space:]]*//; s/^\"//; s/\"$//") || true
      [[ -n "$v" ]] && echo -e "${k^^}\t$v"
    done
  fi
}

apply_config_fields() {
  local line key val
  while IFS=$'\t' read -r key val; do
    [[ -z "$key" ]] && continue
    case "$key" in
      DSN)      [[ -z "$DSN" ]] && DSN="$val" ;;
      HOST)     [[ -z "$HOST" ]] && HOST="$val" ;;
      PORT)     [[ -z "$PORT" ]] && PORT="$val" ;;
      USER)     [[ -z "$USER" ]] && USER="$val" ;;
      PASSWORD) [[ -z "$DBPASS" ]] && DBPASS="$val" ;;
      DBNAME)   [[ -z "$DBNAME" ]] && DBNAME="$val" ;;
      SSLMODE)  [[ -z "$SSLMODE" ]] && SSLMODE="$val" ;;
    esac
  done
}

# ---- 모드 자동 판정 ---------------------------------------------------------
if [[ -z "$MODE" ]]; then
  if [[ -n "$DSN$HOST$USER$DBNAME" || -n "${ARTEX_PG_DSN:-}" || -f "${CONFIG:-config.json}" ]]; then
    MODE="local"
  elif command -v docker >/dev/null 2>&1 && [[ -f docker-compose.yml ]]; then
    MODE="docker"
  else
    MODE="local"
  fi
fi
info "Deployment mode: $MODE"

# ---- 새 비밀번호 수집 -----------------------------------------------------------
if [[ -z "$NEWPASS" ]]; then
  read -r -s -p "Enter a new password (username is fixed as ARTEX):" NEWPASS; echo >&2
  [[ -n "$NEWPASS" ]] || die "Password cannot be empty"
  read -r -s -p "Enter again to confirm:" NEWPASS2; echo >&2
  [[ "$NEWPASS" == "$NEWPASS2" ]] || die "The values do not match"
fi
[[ -n "$NEWPASS" ]] || die "Password cannot be empty"

# 환경 변수로 비밀번호를 psql 에 전달합니다(\getenv 로 읽으며 argv/ps 에 들어가지 않습니다)
export ARTEX_RESET_NEWPASS="$NEWPASS"

# DB 안에서 bcrypt 를 생성해 upsert 합니다. 비밀번호는 :'newpw' 로 자동 이스케이프됩니다. CREATE EXTENSION 은 멱등이며,
# 데이터베이스 역할에 확장 생성 권한이 없으면 여기서 오류가 납니다(안내는 아래 run 의 실패 분기 참조).
SQL=$(cat <<SQL
\\set ON_ERROR_STOP on
\\getenv newpw ARTEX_RESET_NEWPASS
CREATE EXTENSION IF NOT EXISTS pgcrypto;
INSERT INTO settings(key, value)
VALUES ('$PASS_KEY', crypt(:'newpw', gen_salt('bf', $BCRYPT_COST)))
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();
SQL
)

# ---- 실행 -----------------------------------------------------------------
if [[ "$MODE" == "local" ]]; then
  # 접속 정보 우선순위: 명령행 > --dsn/$ARTEX_PG_DSN > config.json
  if [[ -z "$DSN" && -z "$HOST$USER$DBNAME" ]]; then
    [[ -n "${ARTEX_PG_DSN:-}" ]] && DSN="$ARTEX_PG_DSN"
  fi
  if [[ -z "$DSN" && -z "$HOST$USER$DBNAME" ]]; then
    cfg="${CONFIG:-config.json}"
    if [[ -f "$cfg" ]]; then
      info "Reading database settings from $cfg"
      apply_config_fields < <(read_config_json "$cfg")
    fi
  fi

  command -v psql >/dev/null 2>&1 || die "psql not found on this computer (install postgresql-client or use -m docker)"

  declare -a PSQL_ARGS=()
  if [[ -n "$DSN" ]]; then
    PSQL_ARGS=("$DSN")
    target="$DSN"
  else
    [[ -n "$USER"   ]] || die "Database user (-U) is missing, or no valid config.json/DSN is available"
    [[ -n "$DBNAME" ]] || die "Database name (-d) is missing, or no valid config.json/DSN is available"
    HOST="${HOST:-127.0.0.1}"; PORT="${PORT:-5432}"; SSLMODE="${SSLMODE:-disable}"
    PSQL_ARGS=(-h "$HOST" -p "$PORT" -U "$USER" -d "$DBNAME")
    [[ -n "$SSLMODE" ]] && export PGSSLMODE="$SSLMODE"
    [[ -n "$DBPASS" ]] && export PGPASSWORD="$DBPASS"
    target="$USER@$HOST:$PORT/$DBNAME"
  fi

  info "Target database: $target"
  if [[ "$ASSUME_YES" -ne 1 ]]; then
    read -r -p "Reset the ARTEX password in this database? [y/N]" ans
    [[ "$ans" == "y" || "$ans" == "Y" ]] || die "Cancelled"
  fi

  if ! printf '%s\n' "$SQL" | psql "${PSQL_ARGS[@]}" -v ON_ERROR_STOP=1 -q >/dev/null; then
    die "Write failed. For missing pgcrypto or permission errors, use a role allowed to create extensions, or run CREATE EXTENSION pgcrypto manually first."
  fi

else
  # ---- docker ----
  command -v docker >/dev/null 2>&1 || die "docker not found"
  CONTAINER="${CONTAINER:-postgres}"

  # exec 방식 선택: docker compose exec(서비스명)를 우선하고, 아니면 docker exec(컨테이너명)
  if [[ -z "$EXEC_KIND" ]]; then
    if docker compose version >/dev/null 2>&1 && [[ -f docker-compose.yml ]]; then
      EXEC_KIND="compose"
    else
      EXEC_KIND="docker"
    fi
  fi

  # 컨테이너 안 psql 자격 증명: 명령행을 우선, 다음 .env 의 POSTGRES_*, 마지막으로 compose 기본값(artex)
  if [[ -f .env ]]; then
    # shellcheck disable=SC1091
    set -a; . ./.env; set +a
  fi
  DUSER="${USER:-${POSTGRES_USER:-artex}}"
  DNAME="${DBNAME:-${POSTGRES_DB:-artex}}"
  [[ -n "$DBPASS" ]] && export PGPASSWORD="$DBPASS"
  [[ -z "${PGPASSWORD:-}" && -n "${POSTGRES_PASSWORD:-}" ]] && export PGPASSWORD="$POSTGRES_PASSWORD"

  info "Target: psql -U $DUSER -d $DNAME inside container $CONTAINER (exec=$EXEC_KIND)"
  if [[ "$ASSUME_YES" -ne 1 ]]; then
    read -r -p "Reset the ARTEX password in this container database? [y/N]" ans
    [[ "$ans" == "y" || "$ans" == "Y" ]] || die "Cancelled"
  fi

  # -e 는 이름만 주고 값은 주지 않습니다 → 현재 환경에서 상속하므로 비밀번호가 docker 명령 argv 에 나타나지 않습니다.
  declare -a EXEC_CMD
  if [[ "$EXEC_KIND" == "compose" ]]; then
    EXEC_CMD=(docker compose exec -T -e ARTEX_RESET_NEWPASS -e PGPASSWORD "$CONTAINER"
              psql -U "$DUSER" -d "$DNAME" -v ON_ERROR_STOP=1 -q)
  else
    EXEC_CMD=(docker exec -i -e ARTEX_RESET_NEWPASS -e PGPASSWORD "$CONTAINER"
              psql -U "$DUSER" -d "$DNAME" -v ON_ERROR_STOP=1 -q)
  fi

  if ! printf '%s\n' "$SQL" | "${EXEC_CMD[@]}" >/dev/null; then
    die "Write failed. Check the container name (-c), database credentials (POSTGRES_* in .env), and the role permissions for pgcrypto."
  fi
fi

unset ARTEX_RESET_NEWPASS
echo "ARTEX administrator password reset. Log in with username ARTEX and the new password (no service restart required)."
