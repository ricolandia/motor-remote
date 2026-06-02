package model

import "database/sql"

type Node struct {
	ID        string         `json:"id"`
	World     string         `json:"world"`
	SalaID    string         `json:"sala_id"`
	Texto     string         `json:"texto"`
	Tipo      string         `json:"tipo"`
	Tags      []string       `json:"tags"`
	Requisito sql.NullString `json:"requisito"`
	Efeito    sql.NullString `json:"efeito"`
	XP        int            `json:"xp"`
	GMNotes   sql.NullString `json:"gm_notes"`
}

type Choice struct {
	ID            int            `json:"id"`
	NodeOrigem    string         `json:"node_origem"`
	NodeDestino   string         `json:"node_destino"`
	TextoEscolha  string         `json:"texto_escolha"`
	Condicao      sql.NullString `json:"condicao"`
	FlagRequired  sql.NullString `json:"flag_required"`
	Ordem         int            `json:"ordem"`
}
