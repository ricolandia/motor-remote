#!/bin/sh
set -e

# Initialize database if empty
if [ ! -f "$DB_PATH" ] || [ "$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM sqlite_master WHERE type='table';" 2>/dev/null || echo "0")" = "0" ]; then
    echo "[init] Criando banco de dados em $DB_PATH..."
    mkdir -p "$(dirname "$DB_PATH")"
    sqlite3 "$DB_PATH" < /db/schema.sql
    echo "[init] Banco inicializado com sucesso"
fi

echo "[init] Iniciando REMOTE Engine..."
exec /remote-server
