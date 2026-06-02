package game

import (
	"fmt"
	"strings"

	"github.com/ricardo/cli-game/internal/model"
)

type MenuAction int

const (
	MenuSelectWorld MenuAction = iota
	MenuProceed
	MenuError
)

type MenuResult struct {
	Action  MenuAction
	World   string
	Message string
}

func FormatMenu(currentWorld string) string {
	var b strings.Builder
	b.WriteString("\x1b[2J\x1b[H")
	b.WriteString("\x1b[36mCLI-Game: MUD Narrativo\x1b[0m\n\n")
	b.WriteString("Escolha seu mundo:\n\n")

	worlds := []struct {
		key  string
		name string
		desc string
	}{
		{"fantasia", "Fantasia Medieval", "Tavernas, florestas, goblins e magia sutil"},
		{"scifi", "Sci-Fi", "Estacoes orbitais, naves e misterios corporativos"},
		{"horror", "Horror", "Mansoes abandonadas, cultos e terror cosmico"},
		{"posapocalipse", "Pos-Apocalipse", "Ruinas, radiation, gangs e sobrevivencia"},
	}

	for i, w := range worlds {
		marker := " "
		if w.key == currentWorld {
			marker = "\x1b[33m*\x1b[0m"
		}
		b.WriteString(fmt.Sprintf("%s \x1b[33m%d)\x1b[0m \x1b[36m%s\x1b[0m - %s\n", marker, i+1, w.name, w.desc))
	}

	if currentWorld != "" {
		b.WriteString("(\x1b[33m*\x1b[0m=atual Enter=continuar)\n")
	}
	b.WriteString("\x1b[33m>\x1b[0m ")
	return b.String()
}

func ProcessMenuChoice(input string, currentWorld string) *MenuResult {
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" && currentWorld != "" {
		return &MenuResult{Action: MenuProceed, World: currentWorld}
	}

	switch input {
	case "1", "fantasia":
		return &MenuResult{Action: MenuSelectWorld, World: "fantasia"}
	case "2", "scifi":
		return &MenuResult{Action: MenuSelectWorld, World: "scifi"}
	case "3", "horror":
		return &MenuResult{Action: MenuSelectWorld, World: "horror"}
	case "4", "posapocalipse":
		return &MenuResult{Action: MenuSelectWorld, World: "posapocalipse"}
	default:
		return &MenuResult{Action: MenuError, Message: "Opcao invalida. Digite 1 (Fantasia), 2 (Sci-Fi), 3 (Horror), 4 (Pos-Apocalipse)."}
	}
}

func FormatLangMenu() string {
	return "\x1b[2J\x1b[H" +
		"Choose your language / Escolha seu idioma:\n\n" +
		"  \x1b[33m1)\x1b[0m Portugues (PT)\n" +
		"  \x1b[33m2)\x1b[0m English (EN)\n\n" +
		"\x1b[33m>\x1b[0m "
}

func ProcessLangChoice(input string) string {
	input = strings.TrimSpace(input)
	switch input {
	case "1", "pt", "portugues":
		return "pt"
	case "2", "en", "english":
		return "en"
	}
	return ""
}

func GetStartNode(world string) string {
	if n, ok := model.StartNodes[world]; ok && n != "" {
		return n
	}
	return "inicio_fantasia"
}
