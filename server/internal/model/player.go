package model

import "time"

type Player struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	PasswordHash   string    `json:"-"`
	CharName       string    `json:"char_name"`
	World          string    `json:"world"`
	NodeAtual      string    `json:"node_atual"`
	HP             int       `json:"hp"`
	HPMax          int       `json:"hp_max"`
	XP             int       `json:"xp"`
	Level          int       `json:"level"`
	Gold           int       `json:"gold"`
	Charisma       int       `json:"charisma"`
	Persuasion     int       `json:"persuasion"`
	Strength       int       `json:"strength"`
	Agility        int       `json:"agility"`
	History        string    `json:"history"`
	Lang           string    `json:"lang"`
	SupporterTier  string    `json:"supporter_tier"`
	SupporterSince string    `json:"supporter_since"`
	CreatedAt      time.Time `json:"created_at"`
	LastAction     time.Time `json:"last_action"`
}

type Arc struct {
	ID          string `json:"id"`
	World       string `json:"world"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Difficulty  int    `json:"difficulty"`
	StartNode   string `json:"start_node"`
}

type Inventory struct {
	ID         int    `json:"id"`
	PlayerID   string `json:"player_id"`
	Item       string `json:"item"`
	Quantidade int    `json:"quantidade"`
}

type Flag struct {
	PlayerID string `json:"player_id"`
	Flag     string `json:"flag"`
	Valor    string `json:"valor"`
}
