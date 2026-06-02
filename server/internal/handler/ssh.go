package handler

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"golang.org/x/crypto/bcrypt"

	"github.com/ricardo/cli-game/internal/db"
	"github.com/ricardo/cli-game/internal/game"
	"github.com/ricardo/cli-game/internal/session"
)

type SSHServer struct {
	db      *db.DB
	port    int
	session *session.Manager
	keyPath string
}

func NewSSH(database *db.DB, port int, sm *session.Manager) *SSHServer {
	home, _ := os.UserHomeDir()
	keyPath := filepath.Join(home, ".ssh", "cli-game_host_key")
	return &SSHServer{
		db:      database,
		port:    port,
		session: sm,
		keyPath: keyPath,
	}
}

func (s *SSHServer) loadOrGenerateKey() ([]byte, error) {
	if data, err := os.ReadFile(s.keyPath); err == nil {
		log.Printf("Loaded existing host key from %s", s.keyPath)
		return data, nil
	}
	log.Printf("Generating new host key...")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privDER,
	})
	dir := filepath.Dir(s.keyPath)
	if dir != "." {
		os.MkdirAll(dir, 0700)
	}
	if err := os.WriteFile(s.keyPath, pemBytes, 0600); err != nil {
		log.Printf("Warning: could not save host key: %v", err)
	}
	return pemBytes, nil
}

func (s *SSHServer) Start() error {
	pemBytes, err := s.loadOrGenerateKey()
	if err != nil {
		return fmt.Errorf("host key: %w", err)
	}

	ssh.Handle(func(session ssh.Session) {
		s.handleConnection(session)
	})

	log.Printf("SSH server listening on :%d", s.port)
	return ssh.ListenAndServe(
		fmt.Sprintf(":%d", s.port),
		nil,
		ssh.HostKeyPEM(pemBytes),
		ssh.PasswordAuth(func(ctx ssh.Context, password string) bool {
			return s.authenticate(ctx, password)
		}),
	)
}

func (s *SSHServer) authenticate(ctx ssh.Context, password string) bool {
	player, err := s.db.GetPlayerByUsername(ctx.User())
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(player.PasswordHash), []byte(password)) == nil
}

// readLine buffers SSH input until newline (handles character-by-character arrival in PTY)
func readLine(reader io.Reader, buf []byte, timeout time.Duration) (string, error) {
	var lineBuf strings.Builder
	deadline := time.Now().Add(timeout)
	for {
		var n int
		var err error
		if timeout > 0 && time.Now().After(deadline) {
			return "", fmt.Errorf("timeout")
		}
		n, err = reader.Read(buf)
		if err != nil {
			if lineBuf.Len() > 0 {
				return lineBuf.String(), nil
			}
			return "", err
		}
		for i := 0; i < n; i++ {
			ch := buf[i]
			if ch == '\n' || ch == '\r' {
				return lineBuf.String(), nil
			}
			lineBuf.WriteByte(ch)
		}
	}
}

func (s *SSHServer) handleConnection(session ssh.Session) {
	username := session.User()
	player, err := s.db.GetPlayerByUsername(username)
	if err != nil {
		io.WriteString(session, "Erro ao carregar personagem.\n")
		session.Close()
		return
	}

	w := session

	buf := make([]byte, 1024)

	// Language selection (if not set)
	if player.Lang == "" {
		for {
			io.WriteString(w, game.FormatLangMenu())
			n, err := session.Read(buf)
			if err != nil {
				return
			}
			lang := game.ProcessLangChoice(strings.TrimSpace(string(buf[:n])))
			if lang != "" {
				player.Lang = lang
				s.db.UpdatePlayer(player)
				break
			}
			fmt.Fprintf(w, "Opcao invalida.\n")
		}
	}

	fmt.Fprintf(w, "\x1b[36mREMOTE\x1b[0m\n")
	welcomeMsg := "Bem-vindo"
	if player.Lang == "en" {
		welcomeMsg = "Welcome"
	}
	fmt.Fprintf(w, "%s, \x1b[33m%s\x1b[0m!\n", welcomeMsg, player.Username)

	// Menu loop: choose world
	var chosenWorld string
	for {
		io.WriteString(w, game.FormatMenu(player.World))

		n, err := session.Read(buf)
		if err != nil {
			return
		}

		input := strings.TrimSpace(string(buf[:n]))
		result := game.ProcessMenuChoice(input, player.World)

		switch result.Action {
		case game.MenuSelectWorld:
			chosenWorld = result.World
		case game.MenuProceed:
			chosenWorld = player.World
		case game.MenuError:
			fmt.Fprintf(w, "\x1b[31m%s\x1b[0m\n", result.Message)
			continue
		}

		if chosenWorld != "" {
			break
		}
	}

	if chosenWorld != player.World {
		player.World = chosenWorld
		player.NodeAtual = game.GetStartNode(chosenWorld)
		s.db.UpdatePlayer(player)
	}

	eng, err := game.New(s.db, player)
	if err != nil {
		io.WriteString(session, "Erro ao iniciar jogo.\n")
		session.Close()
		return
	}

	s.session.Set(player.ID, eng)
	defer s.session.Remove(player.ID)

	// Game loop
	io.WriteString(w, eng.FormatNodeOutput())
	fmt.Fprintf(w, "\n\x1b[33m>\x1b[0m ")

	for {
		var input string
		readBuf := make([]byte, 1024)

		if eng.IsInCombat() {
			type readRes struct {
				data string
				err  error
			}
			resChan := make(chan readRes, 1)
			go func() {
				n, err := session.Read(readBuf)
				if n > 0 {
					resChan <- readRes{string(readBuf[:n]), err}
				} else {
					resChan <- readRes{"", err}
				}
			}()

			select {
			case res := <-resChan:
				if res.err != nil {
					goto exitLoop
				}
				input = strings.TrimSpace(res.data)
			case <-time.After(15 * time.Second):
				input = "__timeout__"
			}
		} else {
			line, err := readLine(session, readBuf, 0)
			if err != nil {
				break
			}
			input = strings.TrimSpace(line)
		}

		if input == "" && !eng.IsInCombat() && !eng.PostCombat {
			fmt.Fprintf(w, "\n> ")
			continue
		}

		result := eng.ProcessInput(input)
		if result.NeedsRedraw {
			fmt.Fprintf(w, "\x1b[2J\x1b[H")
		}

		if result.Output != "" {
			io.WriteString(w, result.Output)
		}

		if !result.Continue {
			break
		}

		if eng.IsInCombat() {
			fmt.Fprintf(w, "\n  [timer] 15s para agir\n> ")
		} else if result.NeedsRedraw {
			fmt.Fprintf(w, "\n\x1b[33m>\x1b[0m ")
		} else {
			fmt.Fprintf(w, "\n> ")
		}
	}
exitLoop:
}
