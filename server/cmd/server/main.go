package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ricardo/cli-game/internal/db"
	"github.com/ricardo/cli-game/internal/handler"
	"github.com/ricardo/cli-game/internal/session"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "../data/game.db"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("Falha ao conectar banco: %v", err)
	}
	defer database.Close()

	sessionManager := session.NewManager()

	sshServer := handler.NewSSH(database, 2222, sessionManager)
	httpServer := handler.NewHTTP(database, 8080, sessionManager)

	go func() {
		log.Printf("Iniciando servidor HTTP na porta 8080")
		if err := httpServer.Start(); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	go func() {
		log.Printf("Iniciando servidor SSH na porta 2222")
		if err := sshServer.Start(); err != nil {
			log.Fatalf("SSH server error: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Encerrando servidores...")
}
