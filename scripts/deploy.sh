#!/usr/bin/env bash
set -euo pipefail

APP="remote-engine"
SERVER="${1:-}"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

if [ -z "$SERVER" ]; then
  echo "Uso: $0 usuario@vps-ip"
  exit 1
fi

echo "=== REMOTE Engine Deploy ==="

# Compilar
echo "[...] Compilando para linux/amd64..."
cd "$PROJECT_DIR/server"
GOOS=linux GOARCH=amd64 go build -o /tmp/remote-server ./cmd/server
echo "[OK] Binário compilado"

# Backup remoto
ssh "$SERVER" "cp /opt/$APP/data/game.db /opt/$APP/data/game.db.backup.\$(date +%Y%m%d_%H%M%S) 2>/dev/null || true"

# Enviar
echo "[...] Enviando arquivos..."
rsync -avz /tmp/remote-server "$SERVER:/opt/$APP/remote-server"
rsync -avz "$PROJECT_DIR/web/" "$SERVER:/opt/$APP/web/"

# Garantir que banco existe
ssh "$SERVER" "cd /opt/$APP && [ -f data/game.db ] || sqlite3 data/game.db < db/schema.sql"

# Reiniciar
ssh "$SERVER" "sudo systemctl daemon-reload && sudo systemctl restart $APP" || true

echo "[OK] Deploy concluído!"
echo "Web: http://$SERVER:8080"
echo "SSH: ssh -p 2222 <user>@$SERVER"
