package game

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ricardo/cli-game/internal/db"
	"github.com/ricardo/cli-game/internal/model"
)

const (
	XpPerLevel        = 100
	MaxChoicesPerNode = 9
)

type HistoryEntry struct {
	Time     string `json:"t"`
	NodeID   string `json:"n"`
	SalaID   string `json:"s"`
	Escolha  string `json:"e"`
	NodeText string `json:"-"`
}

type Engine struct {
	db            *db.DB
	Player        *model.Player
	node          *model.Node
	Ended         bool
	Combat        *CombatState
	PostCombat    bool
	ChoosingArc   bool
	ChoosingWorld bool
	travelArcs    []model.Arc
	PendingOutput  string
	AwaitingName   bool
	AwaitingWorld  bool
}

func NewPending(database *db.DB, player *model.Player) *Engine {
	return &Engine{db: database, Player: player}
}

func New(database *db.DB, player *model.Player) (*Engine, error) {
	e := &Engine{db: database, Player: player}
	if err := e.loadNode(player.NodeAtual); err != nil {
		return nil, err
	}
	hasChoices, _ := e.hasAvailableChoices()
	e.Ended = !hasChoices
	e.initCombat()
	return e, nil
}

func (e *Engine) startCombatForNode(n *model.Node) {
	enemyHP := 10
	enemySkill := 10
	enemyDR := 0
	victoryNode := ""
	defeatNode := ""

	baseName := n.ID
	if idx := strings.LastIndex(baseName, "_"); idx >= 0 {
		baseName = baseName[:idx]
	}

	for _, tag := range n.Tags {
		if len(tag) == 1 && tag >= "1" && tag <= "9" {
			enemyHP = int(tag[0]-'0') * 3
		}
		if strings.HasPrefix(tag, "vitoria:") {
			victoryNode = strings.TrimPrefix(tag, "vitoria:")
		}
		if strings.HasPrefix(tag, "derrota:") {
			defeatNode = strings.TrimPrefix(tag, "derrota:")
		}
	}

	if victoryNode == "" {
		victoryNode = baseName + "_vitoria"
	}
	if defeatNode == "" {
		defeatNode = baseName + "_derrota"
	}

	enemyName := n.SalaID
	if n.ID != "" {
		parts := strings.Split(n.ID, "_")
		if len(parts) > 1 {
			enemyName = parts[0]
		}
	}

	weaponName, weaponDmg := e.getEquippedWeapon()

	c := &CombatState{}
	c.StartCombat(enemyName, enemyHP, enemySkill, enemyDR, e.Player.HP, e.Player.HPMax, victoryNode, defeatNode)
	c.PlayerWeap = weaponName
	c.PlayerDmg = weaponDmg
	c.SetPlayerStats(e.Player.Strength, e.Player.Agility)
	e.Combat = c
}

func (e *Engine) getEquippedWeapon() (name, dmg string) {
	inv, err := e.db.GetInventory(e.Player.ID)
	if err != nil || len(inv) == 0 {
		return "Mãos Vazias", "1d6-1"
	}
	armas := map[string][2]string{
		"espada":          {"Espada", "1d6+1"},
		"espada_curta":    {"Espada Curta", "1d6+1"},
		"espada_longa":    {"Espada Longa", "1d6+2"},
		"adaga":           {"Adaga", "1d6"},
		"faca":            {"Faca", "1d6"},
		"bisturi":         {"Bisturi", "1d6"},
		"barra_de_ferro":  {"Barra de Ferro", "1d6"},
		"cano":            {"Cano", "1d6"},
		"pistola":         {"Pistola", "1d6+1"},
		"machado":         {"Machado", "1d6+2"},
		"clava":           {"Clava", "1d6"},
		"cassetete":       {"Cassetete", "1d6"},
	}
	for _, item := range inv {
		if w, ok := armas[strings.ToLower(item.Item)]; ok {
			return w[0], w[1]
		}
	}
	return "Mãos Vazias", "1d6-1"
}

func (e *Engine) initCombat() {
	if e.node.Tipo == "combate" {
		e.startCombatForNode(e.node)
	}
}

func (e *Engine) IsInCombat() bool {
	return e.Combat != nil && e.Combat.Active
}

func (e *Engine) loadNode(nodeID string) error {
	resolvedID := e.ResolveID(nodeID)
	n, err := e.db.GetNode(resolvedID)
	if err != nil {
		return err
	}
	// Prevent re-applying effects when loading the same node (e.g. self-loop)
	sameNode := e.node != nil && e.node.ID == resolvedID

	e.node = n
	hasChoices, _ := e.hasAvailableChoices()
	e.Ended = !hasChoices

	// Detect self-loop end node: single available choice pointing to itself
	if !e.Ended {
		choices, _ := e.filteredChoices()
		if len(choices) == 1 && choices[0].NodeDestino == resolvedID {
			e.Ended = true
		}
	}

	if !sameNode && n.Efeito.Valid && n.Efeito.String != "" {
		if err := e.db.ApplyEffects(e.Player, n.Efeito.String); err != nil {
			return err
		}
	}

	if n.Tipo == "combate" && (e.Combat == nil || !e.Combat.Active) {
		e.startCombatForNode(n)
	}
	return nil
}

func (e *Engine) ResolveID(id string) string {
	if e.Player.Lang == "en" && !strings.HasPrefix(id, "en_") {
		return "en_" + id
	}
	return id
}

func (e *Engine) GetNodeText() string { 
	if e.node == nil { return "" }
	return e.node.Texto 
}
func (e *Engine) GetNodeType() string { 
	if e.node == nil { return "" }
	return e.node.Tipo 
}
func (e *Engine) GetSalaID() string {
	if e.node == nil { return "" }  
	return e.node.SalaID 
}

func (e *Engine) LoadSavedPosition() error {
	return e.loadNode(e.Player.NodeAtual)
}

func (e *Engine) SetWorld(world string) error {
	e.Player.World = world
	startNode := GetStartNode(world)
	e.Player.NodeAtual = startNode
	if err := e.db.UpdatePlayer(e.Player); err != nil {
		return err
	}
	return e.loadNode(startNode)
}

type AvailableChoice struct {
	Number int
	Text   string
	Alias  []string
}

func extractAliases(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	if len(words) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var aliases []string
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			aliases = append(aliases, s)
		}
	}
	add(words[0])
	for _, w := range words {
		if w == "norte" || w == "sul" || w == "leste" || w == "oeste" {
			add(w)
		}
	}
	return aliases
}

func (e *Engine) getHistory() []HistoryEntry {
	var h []HistoryEntry
	if e.Player.History == "" || e.Player.History == "[]" {
		return h
	}
	json.Unmarshal([]byte(e.Player.History), &h)
	return h
}

func (e *Engine) filteredChoices() ([]model.Choice, error) {
	choices, err := e.db.GetChoices(e.node.ID)
	if err != nil {
		return nil, err
	}
	var valid []model.Choice
	for _, c := range choices {
		ok, err := e.db.CheckCondition(e.Player, c.Condicao.String)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if c.FlagRequired.Valid && c.FlagRequired.String != "" {
			flagOk, err := e.db.CheckFlag(e.Player.ID, c.FlagRequired.String)
			if err != nil {
				return nil, err
			}
			if !flagOk {
				continue
			}
		}
		valid = append(valid, c)
	}
	return valid, nil
}

func (e *Engine) hasAvailableChoices() (bool, error) {
	choices, err := e.filteredChoices()
	if err != nil {
		return false, err
	}
	return len(choices) > 0, nil
}

func (e *Engine) GetAvailableChoices() ([]AvailableChoice, error) {
	choices, err := e.filteredChoices()
	if err != nil {
		return nil, err
	}
	var available []AvailableChoice
	for i, c := range choices {
		if i >= MaxChoicesPerNode {
			break
		}
		available = append(available, AvailableChoice{
			Number: i + 1, Text: c.TextoEscolha, Alias: extractAliases(c.TextoEscolha),
		})
	}
	return available, nil
}

func (e *Engine) findByAlias(input string) *int {
	choices, err := e.filteredChoices()
	if err != nil {
		return nil
	}
	for i, c := range choices {
		aliases := extractAliases(c.TextoEscolha)
		for _, a := range aliases {
			if strings.ToLower(strings.TrimSpace(input)) == a {
				n := i + 1
				return &n
			}
		}
	}
	return nil
}

func (e *Engine) Execute(number int) (string, error) {
	choices, err := e.filteredChoices()
	if err != nil {
		return "", err
	}

	if number < 1 || number > len(choices) {
		return "", fmt.Errorf("escolha invalida. Digite o numero ou nome da opcao.")
	}

	selected := &choices[number-1]

	e.appendHistory(selected.TextoEscolha)

	e.Player.NodeAtual = selected.NodeDestino
	e.Player.XP += e.node.XP
	e.Player.Level = (e.Player.XP / XpPerLevel) + 1

	if err := e.db.UpdatePlayer(e.Player); err != nil {
		return "", err
	}

	if err := e.loadNode(selected.NodeDestino); err != nil {
		return "", fmt.Errorf("erro ao carregar proximo no: %w", err)
	}

	hasChoices, _ := e.hasAvailableChoices()
	e.Ended = !hasChoices

	return e.node.Texto, nil
}

func (e *Engine) appendHistory(choiceText string) {
	h := e.getHistory()
	h = append(h, HistoryEntry{
		Time:   time.Now().Format("15:04:05"),
		NodeID: e.node.ID,
		SalaID: e.node.SalaID,
		Escolha: func() string {
			if choiceText == "" {
				return "Inicio da jornada"
			}
			return choiceText
		}(),
		NodeText: e.node.Texto,
	})
	data, _ := json.Marshal(h)
	e.Player.History = string(data)
}

func (e *Engine) Restart() error {
	startNode := model.StartNodes[e.Player.World]
	if startNode == "" {
		startNode = "inicio_fantasia"
	}
	if err := e.db.ResetPlayer(e.Player.ID, startNode); err != nil {
		return err
	}
	e.Player.HP = 20
	e.Player.HPMax = 20
	e.Player.XP = 0
	e.Player.Level = 1
	e.Player.Gold = 0
	e.Player.NodeAtual = startNode
	e.Player.History = "[]"
	e.Ended = false
	return e.loadNode(startNode)
}

func (e *Engine) FormatHistory() string {
	h := e.getHistory()
	if len(h) == 0 {
		return "\nNenhum historico disponivel.\n"
	}

	var b strings.Builder
	b.WriteString("\n=== Historico da Jornada ===\n\n")
	for i, entry := range h {
		prefix := fmt.Sprintf("  %2d. [%s] %s", i+1, entry.Time, entry.SalaID)
		if entry.Escolha != "" && entry.Escolha != "Inicio da jornada" {
			b.WriteString(fmt.Sprintf("%s\n      -> %s\n", prefix, entry.Escolha))
		} else {
			b.WriteString(fmt.Sprintf("%s\n      %s\n", prefix, entry.Escolha))
		}
	}
	return b.String()
}

func (e *Engine) FormatEndScreen() string {
	return "\n\x1b[33m[FIM DA JORNADA]\x1b[0m\n\x1b[33m1)\x1b[0m Jogar novamente\n\x1b[33m2)\x1b[0m Menu principal\n\x1b[33m3)\x1b[0m Menu de historias\n"
}

func (e *Engine) FormatDeathScreen() string {
	return fmt.Sprintf("\n\x1b[31m☠ VOCE MORREU ☠\x1b[0m\n\x1b[33m1)\x1b[0m Recomecar (permadeath)\n\x1b[33m2)\x1b[0m Sair\n")
}

func (e *Engine) XPForNextLevel() int {
	return (e.Player.Level) * XpPerLevel
}

func (e *Engine) FormatStatusBar() string {
	invCount := 0
	inv, _ := e.db.GetInventory(e.Player.ID)
	for _, item := range inv {
		invCount += item.Quantidade
	}

	badge := ""
	if e.Player.SupporterTier != "" {
		badge = fmt.Sprintf(" \x1b[33m[%s]\x1b[0m", e.Player.SupporterTier)
	}

	xpNeeded := e.XPForNextLevel()
	name := e.Player.CharName
	if name == "" {
		name = e.Player.Username
	}

	return fmt.Sprintf(
		"\x1b[36m(%s)\x1b[0m \x1b[33m%s\x1b[0m \x1b[36mHP:\x1b[0m%d/%d \x1b[33mAu:\x1b[0m%d \x1b[33mNv:\x1b[0m%d \x1b[33mXP:\x1b[0m%d/%d%s\n",
		e.node.SalaID, name, e.Player.HP, e.Player.HPMax,
		e.Player.Gold, e.Player.Level, e.Player.XP, xpNeeded, badge,
	)
}

func (e *Engine) ApplyPlayerName(texto string) string {
	name := e.Player.CharName
	if name == "" {
		name = e.Player.Username
	}
	return strings.ReplaceAll(texto, "{nome}", name)
}

func (e *Engine) SetCharName(name string) error {
	e.Player.CharName = name
	startNode := GetStartNode(e.Player.World)
	e.Player.NodeAtual = startNode
	if err := e.db.UpdatePlayer(e.Player); err != nil {
		return err
	}
	return e.loadNode(startNode)
}

func (e *Engine) Travel(arcID string) error {
	arc, err := e.db.GetArcByID(arcID)
	if err != nil {
		return fmt.Errorf("arco nao encontrado: %w", err)
	}
	e.Player.World = arc.World
	e.Player.NodeAtual = arc.StartNode
	if err := e.db.TravelPlayer(e.Player.ID, arc.World, arc.StartNode); err != nil {
		return err
	}
	e.Combat = nil
	e.Ended = false
	e.PostCombat = false
	return e.loadNode(arc.StartNode)
}

func (e *Engine) FormatNodeOutput() string {
	if e.IsInCombat() {
		return e.FormatStatusBar() + "\n" + e.Combat.FormatScreen()
	}
	if e.Ended {
		return e.FormatEndScreen()
	}

	var b strings.Builder
	b.WriteString(e.FormatStatusBar())
	b.WriteString("===================================================\n")
	b.WriteString(e.ApplyPlayerName(e.node.Texto))
	b.WriteString("\n")

	choices, err := e.GetAvailableChoices()
	if err != nil || len(choices) == 0 {
		return b.String()
	}

	for _, c := range choices {
		b.WriteString(fmt.Sprintf("\x1b[33m%d)\x1b[0m %s\n", c.Number, c.Text))
	}

	return b.String()
}

func (e *Engine) FormatStatus() string {
	invCount := 0
	inv, _ := e.db.GetInventory(e.Player.ID)
	for _, item := range inv {
		invCount += item.Quantidade
	}

	badge := ""
	if e.Player.SupporterTier != "" {
		badge = fmt.Sprintf(" [%s]", e.Player.SupporterTier)
	}

	return fmt.Sprintf(
		"%s | HP: %d/%d | XP: %d | Nv: %d | Au: %d | Inv: %d | Sala: %s%s",
		e.Player.Username, e.Player.HP, e.Player.HPMax,
		e.Player.XP, e.Player.Level, e.Player.Gold, invCount, e.node.SalaID, badge,
	)
}
