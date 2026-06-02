package game

import (
	"fmt"
	"math/rand"
	"strings"
)

type HitLocation struct {
	Index      int
	Name       string
	Penalty    int
	Effect     string
	DamageMult float64
}

var HitLocations = []HitLocation{
	{1, "Tronco", 0, "dano normal", 1.0},
	{2, "Cabeca", -5, "dano x2, chance de atordoamento", 2.0},
	{3, "Vitals", -3, "dano x3 perfurante", 3.0},
	{4, "Braco D", -2, "pode desarmar", 1.0},
	{5, "Braco E", -2, "pode desarmar", 1.0},
	{6, "Perna D", -2, "pode derrubar", 1.0},
	{7, "Perna E", -2, "pode derrubar", 1.0},
}

type CombatState struct {
	Active           bool
	EnemyName        string
	EnemyHP          int
	EnemyMaxHP       int
	EnemySkill       int
	EnemyDR          int
	PlayerWeap       string
	PlayerDmg        string
	PlayerSkill      int
	PlayerHP         int
	PlayerMaxHP      int
	Turn             int
	TurnResult       string
	InitiativeResult string
	PlayerInit       int
	EnemyInit        int
	VictoryNode      string
	DefeatNode       string
}

type DiceRoll struct {
	Dice  [3]int
	Total int
}

func Roll3D6() DiceRoll {
	var r DiceRoll
	for i := 0; i < 3; i++ {
		r.Dice[i] = rand.Intn(6) + 1
		r.Total += r.Dice[i]
	}
	return r
}

func (c *CombatState) StartCombat(enemyName string, enemyHP, enemySkill, enemyDR, playerHP, playerMaxHP int, victoryNode, defeatNode string) {
	c.Active = true
	c.EnemyName = enemyName
	c.EnemyHP = enemyHP
	c.EnemyMaxHP = enemyHP
	c.EnemySkill = enemySkill
	c.EnemyDR = enemyDR
	c.PlayerWeap = "Espada Curta"
	c.PlayerDmg = "1d6+1"
	c.PlayerSkill = 12
	c.PlayerHP = playerHP
	c.PlayerMaxHP = playerMaxHP
	c.VictoryNode = victoryNode
	c.DefeatNode = defeatNode
	c.Turn = 0
	c.TurnResult = ""
	c.InitiativeResult = ""
	c.RollInitiative()
}

func (c *CombatState) SetPlayerStats(strength, agility int) {
	c.PlayerSkill = 12 + (agility-10)/2
	c.PlayerDmg = fmt.Sprintf("1d6+%d", 1+(strength-10)/2)
}

func (c *CombatState) EndCombat() { c.Active = false }

func (c *CombatState) RollInitiative() {
	c.PlayerInit = c.PlayerHP + c.PlayerSkill
	c.EnemyInit = c.EnemyHP + c.EnemySkill
	if c.PlayerInit >= c.EnemyInit {
		c.InitiativeResult = "VOCE venceu"
	} else {
		c.InitiativeResult = "OPONENTE venceu"
	}
}

func (c *CombatState) PlayerAttack(locIndex int) string {
	if locIndex < 1 || locIndex > len(HitLocations) {
		return "[!] Local invalido."
	}
	loc := HitLocations[locIndex-1]
	adjSkill := c.PlayerSkill + loc.Penalty
	roll := Roll3D6()
	hit := roll.Total <= adjSkill
	critical := roll.Total <= 4
	fumble := roll.Total >= 17

	c.Turn++
	c.RollInitiative()

	var result string
	if fumble {
		result = fmt.Sprintf("[VOCE] %s (%+d) → Rolagem %d vs %d  ✗ FUMBLE!", loc.Name, loc.Penalty, roll.Total, adjSkill)
		return result
	}
	if !hit && !critical {
		result = fmt.Sprintf("[VOCE] %s (%+d) → [%d][%d][%d]=%d vs %d  ✗ ERROU!", loc.Name, loc.Penalty, roll.Dice[0], roll.Dice[1], roll.Dice[2], roll.Total, adjSkill)
		return result
	}

	dmgBonus := 1
	if _, err := fmt.Sscanf(c.PlayerDmg, "1d6+%d", &dmgBonus); err != nil {
		dmgBonus = 1
	}
	dmgRoll := rand.Intn(6) + dmgBonus
	rawDmg := dmgRoll
	if critical {
		rawDmg += rand.Intn(6) + 1
	}
	if loc.DamageMult > 1.0 {
		rawDmg = int(float64(rawDmg) * loc.DamageMult)
	}
	finalDmg := rawDmg - c.EnemyDR
	if finalDmg < 0 {
		finalDmg = 0
	}

	c.EnemyHP -= finalDmg
	if c.EnemyHP < 0 {
		c.EnemyHP = 0
	}

	hitStr := "\x1b[33m✓ ACERTOU!\x1b[0m"
	if critical {
		hitStr = "\x1b[33m✓ CRITICO!\x1b[0m"
	}
	result = fmt.Sprintf("[VOCE] %s (%+d) → [%d][%d][%d]=%d vs %d  %s Dano: %d%s",
		loc.Name, loc.Penalty, roll.Dice[0], roll.Dice[1], roll.Dice[2], roll.Total, adjSkill, hitStr, dmgRoll,
		func() string {
			if loc.DamageMult > 1.0 && finalDmg != dmgRoll {
				return fmt.Sprintf(" x%.0f=%d", loc.DamageMult, rawDmg)
			}
			if c.EnemyDR > 0 { return fmt.Sprintf(" -DR%d=%d", c.EnemyDR, finalDmg) }
			return ""
		}())

	if c.EnemyHP <= 0 {
		result += fmt.Sprintf("  [OPONENTE] HP:0/%d  \x1b[32mDERROTADO!\x1b[0m", c.EnemyMaxHP)
		c.EndCombat()
	}
	return result
}

func (c *CombatState) EnemyAttack() string {
	if !c.Active || c.EnemyHP <= 0 {
		return ""
	}

	loc := HitLocations[rand.Intn(len(HitLocations))]
	adjSkill := c.EnemySkill + loc.Penalty
	roll := Roll3D6()
	hit := roll.Total <= adjSkill
	critical := roll.Total <= 4
	fumble := roll.Total >= 17

	var result string
	if fumble {
		result = fmt.Sprintf("[OPONENTE] %s (%+d) → Rolagem %d vs %d  ✗ FUMBLOU!", loc.Name, loc.Penalty, roll.Total, adjSkill)
		return result
	}
	if !hit && !critical {
		result = fmt.Sprintf("[OPONENTE] → [%d][%d][%d]=%d vs %d  ✗ ERROU!", roll.Dice[0], roll.Dice[1], roll.Dice[2], roll.Total, adjSkill)
		return result
	}

	dmgRoll := rand.Intn(6) + 1
	if critical {
		dmgRoll += rand.Intn(6)
	}
	c.PlayerHP -= dmgRoll
	if c.PlayerHP < 0 {
		c.PlayerHP = 0
	}

	result = fmt.Sprintf("[OPONENTE] → [%d][%d][%d]=%d vs %d  \x1b[31m✓ ACERTOU!\x1b[0m Dano: %d  Seu HP:%d/%d",
		roll.Dice[0], roll.Dice[1], roll.Dice[2], roll.Total, adjSkill, dmgRoll, c.PlayerHP, c.PlayerMaxHP)
	return result
}

func (c *CombatState) FormatScreen() string {
	sep := "\x1b[36m═══════════════════════════════════════════════════════\x1b[0m\n"
	var b strings.Builder

	b.WriteString(sep)
	b.WriteString(fmt.Sprintf("\x1b[33m[INICIATIVA]\x1b[0m %s! (%d x %d)\n", c.InitiativeResult, c.PlayerInit, c.EnemyInit))
	b.WriteString(sep)
	b.WriteString(fmt.Sprintf("\x1b[31m[OPONENTE]\x1b[0m \x1b[33m%s\x1b[0m HP:%d/%d   \x1b[36mVoce\x1b[0m HP:%d/%d Arma:%s\n", c.EnemyName, c.EnemyHP, c.EnemyMaxHP, c.PlayerHP, c.PlayerMaxHP, c.PlayerWeap))
	b.WriteString(sep)
	b.WriteString("Onde atacar?\n")
	for _, loc := range HitLocations {
		penStr := fmt.Sprintf("%+d", loc.Penalty)
		if loc.Penalty == 0 {
			penStr = " 0"
		}
		b.WriteString(fmt.Sprintf("  \x1b[33m%d)\x1b[0m %s (%s) %s\n", loc.Index, loc.Name, penStr, loc.Effect))
	}

	b.WriteString(fmt.Sprintf("\x1b[36m═══════════════════════════════════════════════════════\x1b[0m\n"))

	if c.TurnResult != "" {
		b.WriteString(c.TurnResult)
		b.WriteString("\n")
	}

	b.WriteString("\x1b[33m>\x1b[0m ")
	return b.String()
}
