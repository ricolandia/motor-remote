package game

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ricardo/cli-game/internal/model"
)

type CommandResult struct {
	Output      string
	NeedsRedraw bool
	Continue    bool
}

var builtinCommands = map[string]string{
	"olhar":      "l",
	"olhe":       "l",
	"l":          "l",
	"status":     "st",
	"st":         "st",
	"stats":      "st",
	"inventario": "i",
	"inv":        "i",
	"i":          "i",
	"historico":  "history",
	"hist":       "history",
	"log":        "history",
	"recomecar":  "restart",
	"recomeco":   "restart",
	"reset":      "restart",
	"salvar":     "save",
	"save":       "save",
	"carregar":   "load",
	"load":       "load",
	"viajar":     "travel",
	"travel":     "travel",
	"dimensao":   "travel",
	"migrar":     "travel",
	"menu":       "menu",
	"m":          "menu",
	"equip":      "equip",
	"e":          "equip",
	"ajuda":      "help",
	"aj":         "help",
	"help":       "help",
	"?":          "help",
	"sair":       "quit",
	"s":          "quit",
	"quit":       "quit",
	"exit":       "quit",
}

func (e *Engine) ProcessInput(input string) *CommandResult {
	input = strings.TrimSpace(strings.ToLower(input))
	if e.PostCombat {
		e.PostCombat = false
		return &CommandResult{
			Output:      e.FormatNodeOutput(),
			NeedsRedraw: true,
			Continue:    true,
		}
	}

	if input == "" {
		return &CommandResult{Output: "", NeedsRedraw: false, Continue: true}
	}

	if e.ChoosingArc {
		if num, err := strconv.Atoi(input); err == nil && num >= 0 {
			e.ChoosingArc = false
			if num == 0 {
				e.travelArcs = nil
				return &CommandResult{Output: "Viagem cancelada.\n", NeedsRedraw: false, Continue: true}
			}
			return e.doTravel(num)
		}
		return &CommandResult{Output: "Opcao invalida. Digite um numero ou 0 para voltar.\n", NeedsRedraw: false, Continue: true}
	}

	if e.ChoosingWorld {
		return e.handleWorldMenuInput(input)
	}

	// Handle combat input (all input routed here during combat, before ended check)
	if e.IsInCombat() {
		return e.handleCombatInput(input)
	}

	if e.Ended {
		return e.handleEndGameInput(input)
	}

	// Handle "usar <item>" command
	if strings.HasPrefix(input, "usar ") {
		item := strings.TrimSpace(strings.TrimPrefix(input, "usar "))
		return e.handleUse(item)
	}

	// Check if input is a number (choice selection)
	if num, err := strconv.Atoi(input); err == nil && num > 0 && num <= MaxChoicesPerNode {
		return e.executeChoice(num)
	}

	// Check built-in commands (available in all non-combat states)
	cmd, ok := builtinCommands[input]
	if !ok {
		cmd = input
	}

	switch cmd {
	case "l":
		return &CommandResult{
			Output:      e.FormatNodeOutput(),
			NeedsRedraw: false,
			Continue:    true,
		}
	case "st":
		return &CommandResult{
			Output:      "\n" + e.FormatStatus() + "\n",
			NeedsRedraw: false,
			Continue:    true,
		}
	case "i":
		return e.showInventory()
	case "history":
		return &CommandResult{
			Output:      e.FormatHistory(),
			NeedsRedraw: false,
			Continue:    true,
		}
	case "restart":
		return e.doRestart()
	case "save":
		return e.doSave()
	case "load":
		return e.doLoad()
	case "travel":
		return e.doTravelMenu()
	case "menu":
		return e.showMainMenu()
	case "equip":
		return e.showEquipment()
	case "help":
		return e.showHelp()
	case "quit":
		return &CommandResult{
			Output:      "\nAte logo, aventureiro!\n",
			NeedsRedraw: false,
			Continue:    false,
		}
	}

	// Try alias match
	if numPtr := e.findByAlias(input); numPtr != nil {
		return e.executeChoice(*numPtr)
	}

	return &CommandResult{
		Output:      fmt.Sprintf("\x1b[31mComando desconhecido: '%s'. Digite 'help' para comandos ou digite o numero/alias das opcoes.\x1b[0m", input),
		NeedsRedraw: false,
		Continue:    true,
	}
}

func (e *Engine) handleCombatInput(input string) *CommandResult {
	// Combat timeout - enemy attacks
	if input == "__timeout__" {
		result := e.Combat.EnemyAttack()
		e.Combat.TurnResult = "\x1b[31m[!] TEMPO ESGOTADO!\x1b[0m\n" + result
		return &CommandResult{
			Output:      e.Combat.FormatScreen(),
			NeedsRedraw: true,
			Continue:    true,
		}
	}

	// Check built-in commands during combat
	switch builtinCommands[input] {
	case "l":
		return &CommandResult{Output: e.FormatNodeOutput(), NeedsRedraw: true, Continue: true}
	case "st":
		return &CommandResult{Output: "\n" + e.FormatStatus() + "\n", NeedsRedraw: false, Continue: true}
	case "menu":
		return &CommandResult{Output: e.showMainMenu().Output, NeedsRedraw: false, Continue: true}
	case "i":
		return e.showInventory()
	case "equip":
		return &CommandResult{Output: e.showEquipment().Output, NeedsRedraw: false, Continue: true}
	case "save":
		return &CommandResult{Output: e.doSave().Output, NeedsRedraw: false, Continue: true}
	case "load":
		return &CommandResult{Output: e.doLoad().Output, NeedsRedraw: false, Continue: true}
	case "help":
		return &CommandResult{Output: e.showHelp().Output, NeedsRedraw: false, Continue: true}
	case "quit":
		return &CommandResult{Output: "\nFugiu do combate!\n", NeedsRedraw: false, Continue: false}
	}

	// Parse hit location number
	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(HitLocations) {
		return &CommandResult{
			Output:      fmt.Sprintf("\x1b[31mLocal invalido. Escolha 1-%d.\x1b[0m", len(HitLocations)),
			NeedsRedraw: false,
			Continue:    true,
		}
	}

	// Execute player's attack (returns single-line result)
	playerResult := e.Combat.PlayerAttack(num)

	// Check if enemy defeated
	if !e.Combat.Active {
		// Recupera 50% do HP perdido, nao total
		hpLost := e.Player.HPMax - e.Combat.PlayerHP
		if hpLost > 0 {
			e.Player.HP = e.Combat.PlayerHP + hpLost/2
			if e.Player.HP < 1 {
				e.Player.HP = 1
			}
		} else {
			e.Player.HP = e.Combat.PlayerHP
		}

		nextNode := e.Combat.VictoryNode
		if nextNode == "" {
			nextNode = "vitoria_goblin"
		}
		e.Combat = nil
		return e.forcedTransition(nextNode, "\x1b[33m" + playerResult + "\n[OK] Combate vencido!\x1b[0m\n")
	}

	// Enemy counter-attack (returns single-line result)
	enemyResult := e.Combat.EnemyAttack()

	// Check if player died
	if e.Combat.PlayerHP <= 0 {
		defeatNode := e.Combat.DefeatNode
		if defeatNode == "" {
			defeatNode = "derrota_goblin"
		}
		e.Combat = nil
		return e.forcedTransition(defeatNode, "\x1b[31m☠ VOCE MORREU!\x1b[0m\n" + playerResult + "\n" + enemyResult + "\n")
	}

	// Store turn result and show combat screen
	e.Combat.TurnResult = playerResult + "\n" + enemyResult
	return &CommandResult{
		Output:      e.Combat.FormatScreen(),
		NeedsRedraw: true,
		Continue:    true,
	}
}

func (e *Engine) forcedTransition(nodeID, message string) *CommandResult {
	e.Player.NodeAtual = nodeID
	e.Player.XP += e.node.XP
	e.Player.Level = (e.Player.XP / XpPerLevel) + 1
	if err := e.db.UpdatePlayer(e.Player); err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao salvar.\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	if err := e.loadNode(nodeID); err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao carregar no.\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	return &CommandResult{
		Output:      message + "\n" + e.FormatNodeOutput(),
		NeedsRedraw: true,
		Continue:    true,
	}
}

func (e *Engine) handleEndGameInput(input string) *CommandResult {
	// Built-in commands that work even on end screen
	cmd := builtinCommands[input]
	switch cmd {
	case "save":
		return e.doSave()
	case "load":
		return e.doLoad()
	case "help":
		return e.showHelp()
	case "menu":
		return e.showMainMenu()
	case "st":
		return &CommandResult{Output: "\n" + e.FormatStatus() + "\n", NeedsRedraw: false, Continue: true}
	case "i":
		return e.showInventory()
	case "equip":
		return e.showEquipment()
	case "history":
		return &CommandResult{
			Output:      e.FormatHistory() + "\n" + e.FormatEndScreen(),
			NeedsRedraw: false,
			Continue:    true,
		}
	case "travel":
		return e.doTravelMenu()
	}

	switch {
	case input == "1" || input == "jogar" || input == "novamente" || input == "recomecar" || input == "reset":
		return e.doRestart()
	case input == "2" || input == "menu":
		return e.showMainMenu()
	case input == "3" || input == "historias" || input == "mundo":
		return e.doWorldMenu()
	case input == "sair" || input == "quit":
		return &CommandResult{
			Output:      "\nAte logo, aventureiro!\n",
			NeedsRedraw: false,
			Continue:    false,
		}
	}
	return &CommandResult{
		Output:      "\x1b[31mOpcao invalida.\x1b[0m",
		NeedsRedraw: false,
		Continue:    true,
	}
}

func (e *Engine) executeChoice(num int) *CommandResult {
	texto, err := e.Execute(num)
	if err != nil {
		return &CommandResult{
			Output:      fmt.Sprintf("\x1b[31m%s\x1b[0m", err.Error()),
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	e.node.Texto = texto
	return &CommandResult{
		Output:      e.FormatNodeOutput(),
		NeedsRedraw: true,
		Continue:    true,
	}
}

func (e *Engine) doRestart() *CommandResult {
	if err := e.Restart(); err != nil {
		return &CommandResult{
			Output:      fmt.Sprintf("\x1b[31mErro ao recomecar: %v\x1b[0m", err),
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	return &CommandResult{
		Output:      "[OK] Jornada reiniciada!\n\n" + e.FormatNodeOutput(),
		NeedsRedraw: true,
		Continue:    true,
	}
}

func (e *Engine) showInventory() *CommandResult {
	inv, err := e.db.GetInventory(e.Player.ID)
	if err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao carregar inventario.\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	var b strings.Builder
	b.WriteString("\n\x1b[36mInventario:\x1b[0m\n")
	if len(inv) == 0 {
		b.WriteString("  (vazio)\n")
	} else {
		for _, item := range inv {
			b.WriteString(fmt.Sprintf("  - %s x%d\n", item.Item, item.Quantidade))
		}
	}
	return &CommandResult{
		Output:      b.String(),
		NeedsRedraw: false,
		Continue:    true,
	}
}

func (e *Engine) showHelp() *CommandResult {
	name := e.Player.CharName
	if name == "" {
		name = e.Player.Username
	}

	help := "\n\x1b[36m===== REMOTE — Manual do Jogador =====\x1b[0m\n\n"

	help += "\x1b[33mComo Jogar:\x1b[0m\n"
	help += "  Voce esta em um cenario descrito por texto. Abaixo, opcoes numeradas.\n"
	help += "  Digite o \x1b[33mnumero\x1b[0m ou a \x1b[33mprimeira palavra\x1b[0m e pressione \x1b[33mEnter\x1b[0m.\n"
	help += "  \x1b[31mNo combate\x1b[0m, apenas digite o numero — a acao e imediata, sem Enter.\n\n"

	help += "\x1b[33mComandos:\x1b[0m\n"
	help += "  \x1b[33m<n>\x1b[0m          Escolher opcao numerada\n"
	help += "  \x1b[33m<palavra>\x1b[0m     Digite a 1 palavra da opcao (ex: ir, pegar)\n"
	help += "  \x1b[33ml\x1b[0m /olhar     Reler descricao do local\n"
	help += "  \x1b[33mst\x1b[0m /status   Status completo do personagem\n"
	help += "  \x1b[33mi\x1b[0m /inv      Inventario\n"
	help += "  \x1b[33mequip\x1b[0m /e     Equipamentos (arma, armadura, acessorio)\n"
	help += "  \x1b[33musar\x1b[0m        Usar item do inventario no cenario\n"
	help += "  \x1b[33mmenu\x1b[0m /m      Menu principal\n"
	help += "  \x1b[33mhist\x1b[0m        Historico de decisoes\n"
	help += "  \x1b[33msalvar\x1b[0m     Salvar o jogo\n"
	help += "  \x1b[33mcarregar\x1b[0m   Carregar ultimo save\n"
	help += "  \x1b[33mviajar\x1b[0m      Viajar entre dimensoes (ao final da jornada)\n"
	help += "  \x1b[33mrecomecar\x1b[0m   Reiniciar (permadeath)\n"
	help += "  \x1b[33majuda\x1b[0m /?     Este manual\n"
	help += "  \x1b[33msair\x1b[0m        Desconectar\n\n"

	help += "\x1b[33mCombate (3d6):\x1b[0m\n"
	help += "  \x1b[36m1) Tronco\x1b[0m (0)    Dano normal\n"
	help += "  \x1b[36m2) Cabeca\x1b[0m (-5)   Dano x2, atordoamento\n"
	help += "  \x1b[36m3) Vitals\x1b[0m (-3)   Dano x3 perfurante\n"
	help += "  \x1b[36m4-5) Bracos\x1b[0m (-2) Pode desarmar\n"
	help += "  \x1b[36m6-7) Pernas\x1b[0m (-2) Pode derrubar\n"
	help += "  Critico (<=4)  Fumble (>=17)\n\n"

	help += "\x1b[33mSocial:\x1b[0m\n"
	help += "  Amigavel (0), Racional (-2), Emocional (-2), Agressivo (-3)\n"
	help += "  Use carisma e persuasao para convencer NPCs.\n\n"

	help += "\x1b[33mHistorias Disponiveis:\x1b[0m\n"
	help += "  \x1b[36mFantasia\x1b[0m: O Amuleto de Sylphara — Um amuleto falante, uma fada, um dragao\n"
	help += "  \x1b[36mSci-Fi\x1b[0m: Leviatã — Uma nave misteriosa no sistema solar\n"
	help += "  \x1b[36mHorror\x1b[0m: O Tempo Congelado — O mundo parou e criaturas sugam sangue\n"
	help += "  \x1b[36mPos-Apocalipse\x1b[0m: Projeto Cascos — Um laboratorio, uma cura, uma mutacao\n"
	help += "  Ao final de uma jornada, use \x1b[33mviajar\x1b[0m para migrar entre dimensoes.\n"
	help += "  Seu personagem mantem nivel, itens e atributos.\n\n"

	help += "\x1b[31mPermadeath:\x1b[0m Se HP = 0, seu personagem morre para sempre.\n"
	help += "  Pense antes de agir.\n\n"

	help += "\x1b[36m===== REMOTE =====\x1b[0m\n"

	return &CommandResult{
		Output:      help,
		NeedsRedraw: false,
		Continue:    true,
	}
}

func (e *Engine) doTravelMenu() *CommandResult {
	arcs, err := e.db.GetAllArcs()
	if err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao listar dimensoes.\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	if len(arcs) == 0 {
		return &CommandResult{
			Output:      "\nNenhuma dimensao disponivel para viagem no momento.\n",
			NeedsRedraw: false,
			Continue:    true,
		}
	}

	e.ChoosingArc = true
	e.travelArcs = arcs

	worldOrder := []string{"fantasia", "scifi", "horror", "posapocalipse"}
	worldLabel := map[string]string{
		"fantasia": "\x1b[33m✦ Fantasia\x1b[0m",
		"scifi":    "\x1b[33m✦ Sci-Fi\x1b[0m",
		"horror":   "\x1b[33m✦ Horror\x1b[0m",
		"posapocalipse": "\x1b[33m✦ Pos-Apocalipse\x1b[0m",
	}

	out := "\n\x1b[33m[VIAGEM ENTRE DIMENSOES]\x1b[0m\n"
	out += "Escolha uma aventura:\n\n"
	
	num := 1
	for _, world := range worldOrder {
		worldArcs := []model.Arc{}
		for _, a := range arcs {
			if a.World == world {
				worldArcs = append(worldArcs, a)
			}
		}
		if len(worldArcs) == 0 {
			continue
		}
		out += worldLabel[world] + "\n"
		for _, a := range worldArcs {
			marker := ""
			if a.World == e.Player.World {
				marker = " \x1b[36m(mundo atual)\x1b[0m"
			}
			out += fmt.Sprintf("  \x1b[33m%d)\x1b[0m %s (Dificuldade: %d)%s\n", num, a.Title, a.Difficulty, marker)
			out += fmt.Sprintf("     %s\n", a.Description)
			num++
		}
		out += "\n"
	}
	out += "\x1b[33m0)\x1b[0m Voltar\n"

	return &CommandResult{
		Output:      out,
		NeedsRedraw: false,
		Continue:    true,
	}
}

func (e *Engine) doTravel(arcIndex int) *CommandResult {
	if arcIndex < 1 || arcIndex > len(e.travelArcs) {
		e.ChoosingArc = false
		e.travelArcs = nil
		return &CommandResult{
			Output:      "\x1b[31mOpcao invalida.\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	arc := e.travelArcs[arcIndex-1]
	e.ChoosingArc = false
	e.travelArcs = nil

	if err := e.Travel(arc.ID); err != nil {
		return &CommandResult{
			Output:      fmt.Sprintf("\x1b[31mErro na viagem: %v\x1b[0m", err),
			NeedsRedraw: false,
			Continue:    true,
		}
	}

	return &CommandResult{
		Output:      fmt.Sprintf("\x1b[33m[*] Viajando para %s...\x1b[0m\n\n", arc.Title) + e.FormatNodeOutput(),
		NeedsRedraw: true,
		Continue:    true,
	}
}

func (e *Engine) showMainMenu() *CommandResult {
	out := "\n\x1b[33m[MENU PRINCIPAL]\x1b[0m\n"
	out += "  \x1b[33mst\x1b[0m     - Status do personagem\n"
	out += "  \x1b[33mi\x1b[0m      - Inventario\n"
	out += "  \x1b[33mequip\x1b[0m  - Equipamentos\n"
	out += "  \x1b[33mhist\x1b[0m   - Historico\n"
	out += "  \x1b[33msalvar\x1b[0m - Salvar jogo\n"
	out += "  \x1b[33mcarregar\x1b[0m - Carregar save\n"
	out += "  \x1b[33majuda\x1b[0m  - Manual\n"
	out += "  \x1b[33msair\x1b[0m   - Desconectar\n"
	return &CommandResult{
		Output:      out,
		NeedsRedraw: false,
		Continue:    true,
	}
}

func (e *Engine) showEquipment() *CommandResult {
	out := "\n\x1b[33m[EQUIPAMENTOS]\x1b[0m\n"
	out += "  Arma:     Espada Curta (1d6+1)\n"
	out += "  Armadura: Nenhuma\n"
	out += "  Acessorio: Nenhum\n"
	return &CommandResult{
		Output:      out,
		NeedsRedraw: false,
		Continue:    true,
	}
}

func (e *Engine) handleUse(item string) *CommandResult {
	if e.IsInCombat() {
		return &CommandResult{
			Output:      "\x1b[31mNao pode usar itens durante combate.\x1b[0m\n",
			NeedsRedraw: false,
			Continue:    true,
		}
	}

	// Check if item is in inventory
	inv, err := e.db.GetInventory(e.Player.ID)
	if err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao verificar inventario.\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	hasItem := false
	for _, it := range inv {
		if strings.EqualFold(it.Item, item) && it.Quantidade > 0 {
			hasItem = true
			break
		}
	}
	if !hasItem {
		return &CommandResult{
			Output:      fmt.Sprintf("\x1b[31mVoce nao tem '%s' no inventario.\x1b[0m\n", item),
			NeedsRedraw: false,
			Continue:    true,
		}
	}

	// Look for a choice in current node that uses this item
	choices, err := e.db.GetChoices(e.node.ID)
	if err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao buscar interacoes.\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}

	for _, c := range choices {
		if c.Condicao.Valid && c.Condicao.String == "item:"+item {
			// Found matching choice - execute it
			filtered, _ := e.filteredChoices()
			for i, fc := range filtered {
				if fc.ID == c.ID {
					return e.executeChoice(i + 1)
				}
			}
		}
	}

	return &CommandResult{
		Output:      fmt.Sprintf("\x1b[33mNada para fazer com '%s' aqui.\x1b[0m\n", item),
		NeedsRedraw: false,
		Continue:    true,
	}
}

func (e *Engine) doSave() *CommandResult {
	if err := e.db.UpdatePlayer(e.Player); err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao salvar: %v\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	return &CommandResult{
		Output:      "\x1b[33m[*] Jogo salvo!\x1b[0m\n",
		NeedsRedraw: false,
		Continue:    true,
	}
}

func (e *Engine) doLoad() *CommandResult {
	reloaded, err := e.db.GetPlayerByID(e.Player.ID)
	if err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao carregar: %v\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	e.Player = reloaded
	if err := e.loadNode(e.Player.NodeAtual); err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao carregar no: %v\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	return &CommandResult{
		Output:      "\x1b[33m[*] Jogador carregado do banco!\x1b[0m\n\n" + e.FormatNodeOutput(),
		NeedsRedraw: true,
		Continue:    true,
	}
}

func (e *Engine) doWorldMenu() *CommandResult {
	e.ChoosingWorld = true
	return &CommandResult{
		Output:      FormatMenu(e.Player.World),
		NeedsRedraw: true,
		Continue:    true,
	}
}

func (e *Engine) handleWorldMenuInput(input string) *CommandResult {
	result := ProcessMenuChoice(input, e.Player.World)
	switch result.Action {
	case MenuSelectWorld:
		return e.doRestartInWorld(result.World)
	case MenuProceed:
		e.ChoosingWorld = false
		return &CommandResult{
			Output:      e.FormatNodeOutput(),
			NeedsRedraw: true,
			Continue:    true,
		}
	default:
		return &CommandResult{
			Output:      "\x1b[31m" + result.Message + "\x1b[0m\n",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
}

func (e *Engine) doRestartInWorld(world string) *CommandResult {
	e.ChoosingWorld = false
	startNode := model.StartNodes[world]
	if startNode == "" {
		startNode = "inicio_fantasia"
	}
	if err := e.db.ResetPlayer(e.Player.ID, startNode); err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao reiniciar: " + err.Error() + "\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	e.Player.World = world
	e.Player.HP = 20
	e.Player.HPMax = 20
	e.Player.XP = 0
	e.Player.Level = 1
	e.Player.Gold = 0
	e.Player.NodeAtual = startNode
	e.Player.History = "[]"
	e.Ended = false
	e.Combat = nil
	e.PostCombat = false
	if err := e.db.UpdatePlayer(e.Player); err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao salvar mundo: " + err.Error() + "\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	if err := e.loadNode(startNode); err != nil {
		return &CommandResult{
			Output:      "\x1b[31mErro ao carregar no inicial: " + err.Error() + "\x1b[0m",
			NeedsRedraw: false,
			Continue:    true,
		}
	}
	return &CommandResult{
		Output:      "\x1b[33m[OK] Nova jornada em " + world + "!\x1b[0m\n\n" + e.FormatNodeOutput(),
		NeedsRedraw: true,
		Continue:    true,
	}
}
