CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    world TEXT NOT NULL CHECK(world IN ('fantasia', 'scifi', 'horror', 'posapocalipse')),
    sala_id TEXT NOT NULL,
    texto TEXT NOT NULL,
    tipo TEXT NOT NULL DEFAULT 'historia'
        CHECK(tipo IN ('historia', 'combate', 'comercio', 'enigma', 'evento', 'dialogo')),
    tags TEXT DEFAULT '[]',
    requisito TEXT,
    efeito TEXT,
    xp INTEGER DEFAULT 0,
    gm_notes TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS choices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_origem TEXT NOT NULL REFERENCES nodes(id),
    node_destino TEXT NOT NULL REFERENCES nodes(id),
    texto_escolha TEXT NOT NULL,
    condicao TEXT,
    flag_required TEXT,
    ordem INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS arcs (
    id TEXT PRIMARY KEY,
    world TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    difficulty INTEGER DEFAULT 1,
    start_node TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS players (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    char_name TEXT DEFAULT '',
    world TEXT NOT NULL DEFAULT 'fantasia',
    node_atual TEXT NOT NULL DEFAULT 'inicio_fantasia',
    hp INTEGER DEFAULT 20,
    hp_max INTEGER DEFAULT 20,
    xp INTEGER DEFAULT 0,
    level INTEGER DEFAULT 1,
    gold INTEGER DEFAULT 0,
    charisma INTEGER DEFAULT 10,
    persuasion INTEGER DEFAULT 10,
    strength INTEGER DEFAULT 10,
    agility INTEGER DEFAULT 10,
    history TEXT DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_action TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    supporter_tier TEXT DEFAULT '',
    supporter_since TEXT DEFAULT '',
    lang TEXT DEFAULT 'pt'
);

CREATE TABLE IF NOT EXISTS inventory (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL REFERENCES players(id),
    item TEXT NOT NULL,
    quantidade INTEGER DEFAULT 1,
    UNIQUE(player_id, item)
);

CREATE TABLE IF NOT EXISTS flags (
    player_id TEXT NOT NULL REFERENCES players(id),
    flag TEXT NOT NULL,
    valor TEXT DEFAULT 'true',
    PRIMARY KEY (player_id, flag)
);

CREATE INDEX idx_nodes_world ON nodes(world);
CREATE INDEX idx_nodes_sala ON nodes(sala_id);
CREATE INDEX idx_choices_origem ON choices(node_origem);
CREATE INDEX idx_inventory_player ON inventory(player_id);
CREATE INDEX idx_arcs_world ON arcs(world);
