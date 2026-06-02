package model

var Worlds = []string{"fantasia", "scifi", "horror", "posapocalipse"}

var WorldLabels = map[string]string{
	"fantasia":      "Fantasia Medieval",
	"scifi":         "Sci-Fi",
	"horror":        "Horror",
	"posapocalipse": "Pos-Apocalipse",
}

var WorldDescs = map[string]string{
	"fantasia":      "Tavernas, florestas, goblins e magia sutil",
	"scifi":         "Estacoes orbitais, naves e misterios corporativos",
	"horror":        "Mansoes abandonadas, cultos e terror cosmico",
	"posapocalipse": "Ruinas, radiation, gangs e sobrevivencia",
}

var StartNodes = map[string]string{
	"fantasia":      "taverna_001",
	"scifi":         "nave_001",
	"horror":        "cidade_001",
	"posapocalipse": "deserto_001",
}

var NodeTypes = []string{"historia", "combate", "comercio", "enigma", "evento", "dialogo"}
