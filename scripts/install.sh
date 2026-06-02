#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_NAME="remote-engine"
INSTALL_DIR="/opt/$APP_NAME"
SERVICE_FILE="/etc/systemd/system/$APP_NAME.service"

echo "=== REMOTE Engine — Instalação ==="

# ─── Instalar Go ──────────────────────────────
if ! command -v go &>/dev/null; then
    echo "[...] Instalando Go..."
    wget -q https://go.dev/dl/go1.25.0.linux-amd64.tar.gz -O /tmp/go.tar.gz
    tar -C /usr/local -xzf /tmp/go.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
fi
echo "[OK] Go $(go version | cut -d' ' -f3)"

# ─── Instalar SQLite ──────────────────────────
if ! command -v sqlite3 &>/dev/null; then
    apt-get update -qq && apt-get install -y -qq sqlite3
fi
echo "[OK] SQLite3 disponível"

# ─── Compilar ─────────────────────────────────
echo "[...] Compilando servidor..."
cd "$PROJECT_DIR/server"
go build -o "$PROJECT_DIR/remote-server" ./cmd/server
echo "[OK] Binário compilado: remote-server"

# ─── Instalar ─────────────────────────────────
echo "[...] Instalando em $INSTALL_DIR..."
mkdir -p "$INSTALL_DIR/data" "$INSTALL_DIR/web"
cp "$PROJECT_DIR/remote-server" "$INSTALL_DIR/"
cp -r "$PROJECT_DIR/web/" "$INSTALL_DIR/web/"

# ─── Criar banco vazio ────────────────────────
if [ ! -f "$INSTALL_DIR/data/game.db" ]; then
    echo "[...] Criando banco de dados vazio..."
    sqlite3 "$INSTALL_DIR/data/game.db" < "$PROJECT_DIR/db/schema.sql"
    echo "[OK] Banco vazio criado — use a API /api/register para criar contas"
fi

# ─── Service systemd ──────────────────────────
if [ ! -f "$SERVICE_FILE" ]; then
    echo "[...] Criando serviço systemd..."
    sudo tee "$SERVICE_FILE" << EOF
[Unit]
Description=REMOTE Game Engine
After=network.target

[Service]
ExecStart=$INSTALL_DIR/remote-server
Environment=DB_PATH=$INSTALL_DIR/data/game.db
Environment=STATIC_DIR=$INSTALL_DIR/web/static
Restart=always
User=$(whoami)
Group=$(whoami)
WorkingDirectory=$INSTALL_DIR

[Install]
WantedBy=multi-user.target
EOF
    sudo systemctl daemon-reload
    sudo systemctl enable remote-engine
    echo "[OK] Serviço criado e ativado"
fi

# ─── Iniciar ──────────────────────────────────
sudo systemctl restart remote-engine || true
sleep 2
if systemctl is-active --quiet remote-engine; then
    echo "[OK] Servidor rodando!"
else
    echo "[!] Verifique: sudo journalctl -u remote-engine"
fi

echo ""
echo "=== REMOTE Engine instalado! ==="
echo "Web:  http://$(hostname -I 2>/dev/null | awk '{print $1}'):8080"
echo "SSH:  ssh -p 2222 localhost"
echo ""
echo "Crie um jogador de teste:"
echo '  curl -X POST http://localhost:8080/api/register -H "Content-Type: application/json" -d '\''{"username":"teste","password":"123","char_name":"Aric"}'\'
