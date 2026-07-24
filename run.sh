#!/usr/bin/env bash
# AvtoTest — bitta buyruq bilan butun loyihani ishga tushiradi:
# Docker infra (postgres/redis/minio) + backend (Go, :8090) + frontend (Next.js, :3000).
#
# Ishlatish:
#   ./run.sh              — hammasini ishga tushiradi
#   ./run.sh --stop-infra — Ctrl+C bosilganda docker infra ham to'xtaydi (default: infra ishlab qoladi)
#
# To'xtatish: Ctrl+C (backend to'xtaydi; frontend ham Ctrl+C bilan birga to'xtaydi).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"

BACKEND_PORT=8090
FRONTEND_PORT=3000
LOG_DIR="$SCRIPT_DIR/.run"
mkdir -p "$LOG_DIR"
BACKEND_LOG="$LOG_DIR/backend.log"
BACKEND_PID=""

STOP_INFRA=0
if [[ "${1:-}" == "--stop-infra" ]]; then
  STOP_INFRA=1
fi

cleanup() {
  echo
  echo "To'xtatilmoqda..."
  if [[ -n "$BACKEND_PID" ]] && kill -0 "$BACKEND_PID" 2>/dev/null; then
    kill "$BACKEND_PID" 2>/dev/null || true
    wait "$BACKEND_PID" 2>/dev/null || true
  fi
  if [[ "$STOP_INFRA" -eq 1 ]]; then
    echo "Docker infra to'xtatilmoqda (docker compose down)..."
    docker compose down >/dev/null 2>&1 || true
  else
    echo "Docker infra (postgres/redis/minio) ishlab qolmoqda — keyingi safar tezroq ishga tushadi."
    echo "To'liq to'xtatish uchun: ./run.sh --stop-infra  (yoki: docker compose down)"
  fi
  echo "To'xtatildi."
}
trap cleanup EXIT INT TERM

command -v docker >/dev/null || { echo "XATO: docker topilmadi."; exit 1; }
command -v go >/dev/null || { echo "XATO: go topilmadi ($HOME/.local/go/bin/go kutilgan edi)."; exit 1; }
command -v npm >/dev/null || { echo "XATO: npm topilmadi."; exit 1; }

port_busy() {
  ss -ltnp 2>/dev/null | grep -q ":$1 "
}

if port_busy "$BACKEND_PORT"; then
  echo "XATO: $BACKEND_PORT port band. Eski jarayonni toping va to'xtating: ss -ltnp | grep :$BACKEND_PORT"
  exit 1
fi
if port_busy "$FRONTEND_PORT"; then
  echo "XATO: $FRONTEND_PORT port band. Eski jarayonni toping va to'xtating: ss -ltnp | grep :$FRONTEND_PORT"
  exit 1
fi

echo "1/4 — Docker infra (postgres, redis, minio) ishga tushirilmoqda..."
# --wait ishlatilmaydi: minio-init bir martalik konteyner, vazifasini
# bajarib (muvaffaqiyatli) chiqib ketadi — docker compose buni "running
# emas" deb hisoblab --wait'ni xato kod bilan tugatadi. Shuning uchun
# postgres tayyorligini o'zimiz alohida tekshiramiz (backend faqat shu DB'ga muhtoj).
docker compose up -d
printf "   Postgres tayyor bo'lishini kutmoqda"
pg_ready=0
for _ in $(seq 1 30); do
  if docker compose exec -T postgres pg_isready -U avtotest >/dev/null 2>&1; then
    pg_ready=1
    break
  fi
  printf "."
  sleep 1
done
echo
if [[ "$pg_ready" -ne 1 ]]; then
  echo "XATO: postgres 30 soniyada tayyor bo'lmadi. 'docker compose ps' bilan tekshiring."
  exit 1
fi

echo "2/4 — Frontend paketlari tekshirilmoqda..."
if [[ ! -d "$SCRIPT_DIR/frontend/node_modules" ]]; then
  echo "   node_modules topilmadi — npm install ishga tushirilmoqda (bir martalik, biroz vaqt oladi)..."
  (cd "$SCRIPT_DIR/frontend" && npm install)
fi

echo "3/4 — Backend ishga tushirilmoqda (port $BACKEND_PORT; DB migratsiyalar avtomatik qo'llanadi)..."
(cd "$SCRIPT_DIR/backend" && PORT="$BACKEND_PORT" ENV=dev go run ./cmd/api) > "$BACKEND_LOG" 2>&1 &
BACKEND_PID=$!

printf "   Backend tayyor bo'lishini kutmoqda"
ready=0
for _ in $(seq 1 30); do
  if curl -fsS "http://localhost:$BACKEND_PORT/healthz" >/dev/null 2>&1; then
    ready=1
    break
  fi
  if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
    break
  fi
  printf "."
  sleep 1
done
echo

if [[ "$ready" -ne 1 ]]; then
  echo "XATO: backend $BACKEND_PORT portda javob bermadi. Oxirgi loglar:"
  tail -n 40 "$BACKEND_LOG"
  exit 1
fi
echo "   Backend tayyor."

echo "4/4 — Frontend ishga tushirilmoqda (port $FRONTEND_PORT)..."
echo
echo "=================================================================="
echo "  Frontend:      http://localhost:$FRONTEND_PORT"
echo "  Backend API:   http://localhost:$BACKEND_PORT/healthz"
echo "  Backend log:   $BACKEND_LOG"
echo "  MinIO konsol:  http://localhost:9001  (login: avtotest / avtotest123)"
echo
echo "  Eslatma: Payme/Click sandbox kalitlari hali ENV'da yo'q — to'lov"
echo "  webhook'lari hozircha har doim autentifikatsiya xatosi qaytaradi,"
echo "  bu KUTILGAN holat (kalitlar qo'yilmaguncha)."
echo
echo "  To'xtatish uchun: Ctrl+C"
echo "=================================================================="
echo

(cd "$SCRIPT_DIR/frontend" && npm run dev)
