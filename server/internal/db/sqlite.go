package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/ricardo/cli-game/internal/model"
)

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

// ─── Nodes ───────────────────────────────────────────────

func (d *DB) GetNodesByWorld(world string) ([]model.Node, error) {
	rows, err := d.conn.Query(
		`SELECT id, world, sala_id, texto, tipo, tags, requisito, efeito, xp, gm_notes
		 FROM nodes WHERE world = ? ORDER BY id`, world)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		var tagsJSON string
		if err := rows.Scan(&n.ID, &n.World, &n.SalaID, &n.Texto, &n.Tipo,
			&tagsJSON, &n.Requisito, &n.Efeito, &n.XP, &n.GMNotes); err != nil {
			return nil, err
		}
		if tagsJSON != "" {
			json.Unmarshal([]byte(tagsJSON), &n.Tags)
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (d *DB) GetNode(id string) (*model.Node, error) {
	row := d.conn.QueryRow(
		`SELECT id, world, sala_id, texto, tipo, tags, requisito, efeito, xp, gm_notes
		 FROM nodes WHERE id = ?`, id)

	var n model.Node
	var tagsJSON string
	err := row.Scan(&n.ID, &n.World, &n.SalaID, &n.Texto, &n.Tipo,
		&tagsJSON, &n.Requisito, &n.Efeito, &n.XP, &n.GMNotes)
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", id, err)
	}
	if tagsJSON != "" {
		json.Unmarshal([]byte(tagsJSON), &n.Tags)
	}
	return &n, nil
}

func (d *DB) GetChoices(nodeOrigem string) ([]model.Choice, error) {
	rows, err := d.conn.Query(
		`SELECT id, node_origem, node_destino, texto_escolha, condicao, flag_required, ordem
		 FROM choices WHERE node_origem = ? ORDER BY ordem`, nodeOrigem)
	if err != nil {
		return nil, fmt.Errorf("get choices for %s: %w", nodeOrigem, err)
	}
	defer rows.Close()

	var choices []model.Choice
	for rows.Next() {
		var c model.Choice
		if err := rows.Scan(&c.ID, &c.NodeOrigem, &c.NodeDestino,
			&c.TextoEscolha, &c.Condicao, &c.FlagRequired, &c.Ordem); err != nil {
			return nil, fmt.Errorf("scan choice: %w", err)
		}
		choices = append(choices, c)
	}
	return choices, nil
}

// ─── Arcs ────────────────────────────────────────────────

func (d *DB) CreateArc(a *model.Arc) error {
	_, err := d.conn.Exec(
		`INSERT INTO arcs (id, world, title, description, difficulty, start_node)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, a.World, a.Title, a.Description, a.Difficulty, a.StartNode)
	return err
}

func (d *DB) GetArcsByWorld(world string) ([]model.Arc, error) {
	rows, err := d.conn.Query(
		`SELECT id, world, title, description, difficulty, start_node
		 FROM arcs WHERE world = ? ORDER BY difficulty`, world)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var arcs []model.Arc
	for rows.Next() {
		var a model.Arc
		if err := rows.Scan(&a.ID, &a.World, &a.Title, &a.Description, &a.Difficulty, &a.StartNode); err != nil {
			return nil, err
		}
		arcs = append(arcs, a)
	}
	return arcs, nil
}

func (d *DB) GetAllArcs() ([]model.Arc, error) {
	rows, err := d.conn.Query(
		`SELECT id, world, title, description, difficulty, start_node
		 FROM arcs ORDER BY world, difficulty`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var arcs []model.Arc
	for rows.Next() {
		var a model.Arc
		if err := rows.Scan(&a.ID, &a.World, &a.Title, &a.Description, &a.Difficulty, &a.StartNode); err != nil {
			return nil, err
		}
		arcs = append(arcs, a)
	}
	return arcs, nil
}

func (d *DB) GetArcByID(id string) (*model.Arc, error) {
	row := d.conn.QueryRow(
		`SELECT id, world, title, description, difficulty, start_node
		 FROM arcs WHERE id = ?`, id)
	var a model.Arc
	err := row.Scan(&a.ID, &a.World, &a.Title, &a.Description, &a.Difficulty, &a.StartNode)
	if err != nil {
		return nil, fmt.Errorf("get arc %s: %w", id, err)
	}
	return &a, nil
}

// ─── Players ─────────────────────────────────────────────

func (d *DB) CreatePlayer(p *model.Player) error {
	_, err := d.conn.Exec(
		`INSERT INTO players (id, username, password_hash, char_name, world, node_atual, hp, hp_max, lang)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Username, p.PasswordHash, p.CharName, p.World, p.NodeAtual, p.HP, p.HPMax, p.Lang)
	if err != nil {
		return fmt.Errorf("create player: %w", err)
	}
	return nil
}

func scanPlayer(row *sql.Row) (*model.Player, error) {
	var p model.Player
	err := row.Scan(&p.ID, &p.Username, &p.PasswordHash, &p.CharName, &p.World, &p.NodeAtual,
		&p.HP, &p.HPMax, &p.XP, &p.Level, &p.Gold,
		&p.Charisma, &p.Persuasion, &p.Strength, &p.Agility,
		&p.History, &p.Lang, &p.SupporterTier, &p.SupporterSince, &p.CreatedAt, &p.LastAction)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

const playerCols = `id, username, password_hash, char_name, world, node_atual, hp, hp_max,
 xp, level, gold, charisma, persuasion, strength, agility,
 history, lang, supporter_tier, supporter_since, created_at, last_action`

func (d *DB) GetPlayerByUsername(username string) (*model.Player, error) {
	row := d.conn.QueryRow(
		`SELECT `+playerCols+` FROM players WHERE username = ?`, username)
	return scanPlayer(row)
}

func (d *DB) GetPlayerByID(id string) (*model.Player, error) {
	row := d.conn.QueryRow(
		`SELECT `+playerCols+` FROM players WHERE id = ?`, id)
	return scanPlayer(row)
}

func (d *DB) UpdatePlayer(p *model.Player) error {
	_, err := d.conn.Exec(
		`UPDATE players SET char_name=?, world=?, node_atual=?, hp=?, hp_max=?, xp=?, level=?,
		                    gold=?, charisma=?, persuasion=?, strength=?, agility=?,
		                    history=?, lang=?, last_action=CURRENT_TIMESTAMP
		 WHERE id=?`,
		p.CharName, p.World, p.NodeAtual, p.HP, p.HPMax, p.XP, p.Level,
		p.Gold, p.Charisma, p.Persuasion, p.Strength, p.Agility,
		p.History, p.Lang, p.ID)
	if err != nil {
		return fmt.Errorf("update player: %w", err)
	}
	return nil
}

func (d *DB) ResetPlayer(playerID, startNode string) error {
	_, err := d.conn.Exec(
		`UPDATE players SET node_atual=?, hp=20, hp_max=20, xp=0, level=1,
		                    gold=0, history='[]', last_action=CURRENT_TIMESTAMP
		 WHERE id=?`,
		startNode, playerID)
	if err != nil {
		return fmt.Errorf("reset player: %w", err)
	}
	_, err = d.conn.Exec(`DELETE FROM inventory WHERE player_id=?`, playerID)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(`DELETE FROM flags WHERE player_id=?`, playerID)
	return err
}

func (d *DB) TravelPlayer(playerID, world, startNode string) error {
	_, err := d.conn.Exec(
		`UPDATE players SET world=?, node_atual=?, last_action=CURRENT_TIMESTAMP
		 WHERE id=?`, world, startNode, playerID)
	return err
}

func (d *DB) UpdateSupporter(playerID, tier, since string) error {
	_, err := d.conn.Exec(
		`UPDATE players SET supporter_tier=?, supporter_since=?
		 WHERE id=?`, tier, since, playerID)
	return err
}

// ─── Inventory ───────────────────────────────────────────

func (d *DB) GetInventory(playerID string) ([]model.Inventory, error) {
	rows, err := d.conn.Query(
		`SELECT id, player_id, item, quantidade
		 FROM inventory WHERE player_id = ?`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var inv []model.Inventory
	for rows.Next() {
		var i model.Inventory
		if err := rows.Scan(&i.ID, &i.PlayerID, &i.Item, &i.Quantidade); err != nil {
			return nil, err
		}
		inv = append(inv, i)
	}
	return inv, nil
}

func (d *DB) AddItem(playerID, item string, qty int) error {
	_, err := d.conn.Exec(
		`INSERT INTO inventory (player_id, item, quantidade)
		 VALUES (?, ?, ?)
		 ON CONFLICT(player_id, item) DO UPDATE SET quantidade = quantidade + ?`,
		playerID, item, qty, qty)
	return err
}

func (d *DB) RemoveItem(playerID, item string, qty int) error {
	_, err := d.conn.Exec(
		`UPDATE inventory SET quantidade = quantidade - ?
		 WHERE player_id = ? AND item = ? AND quantidade >= ?`,
		qty, playerID, item, qty)
	return err
}

// ─── Flags ───────────────────────────────────────────────

func (d *DB) GetFlag(playerID, flag string) (string, error) {
	row := d.conn.QueryRow(
		`SELECT valor FROM flags WHERE player_id = ? AND flag = ?`,
		playerID, flag)
	var valor string
	err := row.Scan(&valor)
	if err != nil {
		return "", err
	}
	return valor, nil
}

func (d *DB) SetFlag(playerID, flag, valor string) error {
	_, err := d.conn.Exec(
		`INSERT INTO flags (player_id, flag, valor)
		 VALUES (?, ?, ?)
		 ON CONFLICT(player_id, flag) DO UPDATE SET valor = ?`,
		playerID, flag, valor, valor)
	return err
}

// ─── Effects ─────────────────────────────────────────────

func (d *DB) ApplyEffects(player *model.Player, efeito string) error {
	if efeito == "" {
		return nil
	}
	parts := strings.Split(efeito, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := kv[0]
		val := kv[1]

		switch key {
		case "hp":
			var delta int
			fmt.Sscanf(val, "%d", &delta)
			player.HP += delta
			if player.HP > player.HPMax {
				player.HP = player.HPMax
			}
			if player.HP < 0 {
				player.HP = 0
			}
		case "gold":
			var delta int
			fmt.Sscanf(val, "%d", &delta)
			player.Gold += delta
			if player.Gold < 0 {
				player.Gold = 0
			}
		case "xp":
			var delta int
			fmt.Sscanf(val, "%d", &delta)
			player.XP += delta
			if player.XP < 0 {
				player.XP = 0
			}
		case "item":
			if err := d.AddItem(player.ID, strings.ToLower(val), 1); err != nil {
				return err
			}
		case "flag":
			if err := d.SetFlag(player.ID, val, "true"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *DB) CheckCondition(player *model.Player, condicao string) (bool, error) {
	if condicao == "" {
		return true, nil
	}
	parts := strings.SplitN(condicao, ":", 2)
	if len(parts) != 2 {
		return false, nil
	}
	key := parts[0]
	val := parts[1]

	switch key {
	case "gold":
		var needed int
		fmt.Sscanf(val, "%d", &needed)
		return player.Gold >= needed, nil
	case "hp":
		var needed int
		fmt.Sscanf(val, "%d", &needed)
		return player.HP >= needed, nil
	case "item":
		inv, err := d.GetInventory(player.ID)
		if err != nil {
			return false, err
		}
		for _, i := range inv {
			if i.Item == val && i.Quantidade > 0 {
				return true, nil
			}
		}
		return false, nil
	case "level":
		var needed int
		fmt.Sscanf(val, "%d", &needed)
		return player.Level >= needed, nil
	case "charisma":
		var needed int
		fmt.Sscanf(val, "%d", &needed)
		return player.Charisma >= needed, nil
	case "persuasion":
		var needed int
		fmt.Sscanf(val, "%d", &needed)
		return player.Persuasion >= needed, nil
	case "strength":
		var needed int
		fmt.Sscanf(val, "%d", &needed)
		return player.Strength >= needed, nil
	case "agility":
		var needed int
		fmt.Sscanf(val, "%d", &needed)
		return player.Agility >= needed, nil
	default:
		return true, nil
	}
}

func (d *DB) CheckFlag(playerID, flag string) (bool, error) {
	if flag == "" {
		return true, nil
	}
	_, err := d.GetFlag(playerID, flag)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
