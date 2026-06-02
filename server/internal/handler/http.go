package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"

	"github.com/ricardo/cli-game/internal/db"
	"github.com/ricardo/cli-game/internal/game"
	"github.com/ricardo/cli-game/internal/model"
	"github.com/ricardo/cli-game/internal/session"
)

func resolveStaticDir() string {
	if d := os.Getenv("STATIC_DIR"); d != "" {
		return d
	}
	// Try from CWD (project root) first, then from server/ subdir
	candidates := []string{"web/static", "../web/static"}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			log.Printf("Static dir: %s", abs)
			return c
		}
	}
	return "web/static"
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type rateEntry struct {
	count int
	reset time.Time
}

type HTTPServer struct {
	db      *db.DB
	port    int
	session *session.Manager

	registerLimits map[string]*rateEntry
	registerMu     sync.Mutex
}

func NewHTTP(database *db.DB, port int, sm *session.Manager) *HTTPServer {
	return &HTTPServer{
		db:             database,
		port:           port,
		session:        sm,
		registerLimits: make(map[string]*rateEntry),
	}
}

func (h *HTTPServer) resolveWebDir(sub string) string {
	if d := os.Getenv("STATIC_DIR"); d != "" {
		return filepath.Join(d, "..", sub)
	}
	candidates := []string{
		filepath.Join("web", sub),
		filepath.Join("..", "web", sub),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return filepath.Join("web", sub)
}

func (h *HTTPServer) Start() error {
	mux := http.NewServeMux()
	staticDir := resolveStaticDir()
	landingDir := h.resolveWebDir("landing")
	playDir := h.resolveWebDir("play")

	// Serve static files (CSS, JS for old terminal)
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// WebSocket (existing)
	mux.HandleFunc("/ws", h.handleWebSocket)

	// API: register
	mux.HandleFunc("/api/register", h.handleRegister)

	// API: login
	mux.HandleFunc("/api/login", h.handleLogin)

	// API: command
	mux.HandleFunc("/api/command", h.handleCommand)

	// API: export
	mux.HandleFunc("/api/export", h.handleExport)

	// API: set-supporter (protegido)
	mux.HandleFunc("/api/set-supporter", h.handleSetSupporter)

	// API: story graph
	mux.HandleFunc("/api/story-graph", h.handleStoryGraph)

	// API: save story
	mux.HandleFunc("/api/save-story", h.handleSaveStory)

	// API: supporter info
	mux.HandleFunc("/api/supporter", h.handleSupporter)

	// Assinar page
	assinarPath := filepath.Join(staticDir, "assinar.html")
	mux.HandleFunc("/assinar", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, assinarPath)
	})

	// Play interface
	mux.Handle("/play/", http.StripPrefix("/play/", http.FileServer(http.Dir(playDir))))
	mux.HandleFunc("/play", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(playDir, "index.html"))
	})

	// Manual page
	manualPath := filepath.Join(playDir, "manual.html")
	mux.HandleFunc("/manual", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, manualPath)
	})
	mux.HandleFunc("/ajuda", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, manualPath)
	})

	// Editor page
	editorDir := h.resolveWebDir("editor")
	mux.Handle("/editor/", http.StripPrefix("/editor/", http.FileServer(http.Dir(editorDir))))
	mux.HandleFunc("/editor", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(editorDir, "index.html"))
	})

	// Landing page
	landingIndex := filepath.Join(landingDir, "index.html")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, landingIndex)
	})

	log.Printf("HTTP server listening on :%d", h.port)
	return http.ListenAndServe(fmt.Sprintf(":%d", h.port), mux)
}

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	World    string `json:"world"`
	CharName string `json:"char_name"`
	Lang     string `json:"lang"`
}

func (h *HTTPServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}

	// Rate limit: max 3 registrations per hour per IP
	h.registerMu.Lock()
	ip := strings.Split(r.RemoteAddr, ":")[0]
	entry, ok := h.registerLimits[ip]
	now := time.Now()
	if !ok || now.After(entry.reset) {
		entry = &rateEntry{count: 0, reset: now.Add(time.Hour)}
		h.registerLimits[ip] = entry
	}
	if entry.count >= 3 {
		h.registerMu.Unlock()
		http.Error(w, "muitas tentativas. Aguarde antes de criar outra conta.", http.StatusTooManyRequests)
		return
	}
	entry.count++
	h.registerMu.Unlock()

	if req.World == "" {
		req.World = "fantasia"
	}

	// Check if world is valid
	valid := false
	for _, w := range model.Worlds {
		if w == req.World {
			valid = true
			break
		}
	}
	if !valid {
		req.World = "fantasia"
	}

	// Check if node exists for this world
	startNode := model.StartNodes[req.World]
	if startNode == "" {
		startNode = "inicio_fantasia"
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}

	// Generate simple UUID-like ID
	id := fmt.Sprintf("p_%s_%d", req.Username, len(req.Username))

	charName := req.CharName
	if charName == "" {
		charName = req.Username
	}

	lang := req.Lang
	if lang != "en" {
		lang = "pt"
	}

	player := &model.Player{
		ID:           id,
		Username:     req.Username,
		PasswordHash: string(hash),
		CharName:     charName,
		World:        req.World,
		NodeAtual:    startNode,
		HP:           20,
		HPMax:        20,
		XP:           0,
		Level:        1,
		Gold:         0,
		Lang:         lang,
	}

	if err := h.db.CreatePlayer(player); err != nil {
		http.Error(w, fmt.Sprintf("erro ao criar: %v", err), http.StatusConflict)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Personagem criado! Faça login.",
	})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *HTTPServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	player, err := h.db.GetPlayerByUsername(req.Username)
	if err != nil {
		http.Error(w, "usuario ou senha invalidos", http.StatusUnauthorized)
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(player.PasswordHash), []byte(req.Password)) != nil {
		http.Error(w, "usuario ou senha invalidos", http.StatusUnauthorized)
		return
	}

	// Create engine in pre-game state (shows menu first, no world loaded)
	eng := game.NewPending(h.db, player)
	eng.AwaitingWorld = true
	eng.PendingOutput = game.FormatMenu(player.World)
	h.session.Set(player.ID, eng)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"token":     player.ID,
		"username":  player.Username,
		"char_name": player.CharName,
		"lang":      player.Lang,
		"world":     player.World,
		"node_text": eng.GetNodeText(),
	})
}

type commandRequest struct {
	Token string `json:"token"`
	Input string `json:"input"`
}

type commandResponse struct {
	Output  string `json:"output"`
	Redraw  bool   `json:"redraw"`
	Continue bool  `json:"continue"`
}

func stripANSI(s string) string {
	r := strings.NewReplacer(
		"\x1b[31m", "", "\x1b[32m", "", "\x1b[33m", "", "\x1b[36m", "",
		"\x1b[0m", "", "\x1b[2J", "", "\x1b[H", "",
	)
	return r.Replace(s)
}

func (h *HTTPServer) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req commandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	eng := h.session.Get(req.Token)
	if eng == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	// Handle awaiting world choice (menu)
	if eng.AwaitingWorld {
		// Empty input: show menu for new players, proceed for returning
		if strings.TrimSpace(req.Input) == "" {
			if eng.Player.CharName != "" && eng.Player.World != "" {
				// Returning player: proceed directly without changing position
				eng.AwaitingWorld = false
				eng.PendingOutput = ""
				if err := eng.LoadSavedPosition(); err != nil {
					eng.SetWorld(eng.Player.World)
				}
				output := stripANSI(eng.FormatNodeOutput())
				resp := commandResponse{Output: output, Redraw: true, Continue: true}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}
			// New player: show menu
			output := stripANSI(game.FormatMenu(eng.Player.World))
			resp := commandResponse{Output: output, Redraw: true, Continue: true}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		menuResult := game.ProcessMenuChoice(req.Input, eng.Player.World)
		switch menuResult.Action {
		case game.MenuSelectWorld:
			eng.AwaitingWorld = false
			eng.PendingOutput = ""
			eng.SetWorld(menuResult.World)
			if eng.Player.CharName == "" {
				eng.AwaitingName = true
				eng.PendingOutput = "Digite seu nome, aventureiro: "
				resp := commandResponse{Output: "Digite seu nome, aventureiro: ", Redraw: true, Continue: true}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			} else {
				output := stripANSI(eng.FormatNodeOutput())
				resp := commandResponse{Output: output, Redraw: true, Continue: true}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}
			return
		case game.MenuProceed:
			eng.AwaitingWorld = false
			eng.PendingOutput = ""
			eng.SetWorld(menuResult.World)
			if eng.Player.CharName == "" {
				eng.AwaitingName = true
				eng.PendingOutput = "Digite seu nome, aventureiro: "
				resp := commandResponse{Output: "Digite seu nome, aventureiro: ", Redraw: true, Continue: true}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			} else {
				output := stripANSI(eng.FormatNodeOutput())
				resp := commandResponse{Output: output, Redraw: true, Continue: true}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}
			return
		case game.MenuError:
			resp := commandResponse{Output: menuResult.Message, Redraw: false, Continue: true}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
	}

	// Handle awaiting character name (check before PendingOutput)
	if eng.AwaitingName {
		if strings.TrimSpace(req.Input) == "" && eng.PendingOutput != "" {
			// Show name prompt again
			output := stripANSI(eng.PendingOutput)
			resp := commandResponse{Output: output, Redraw: true, Continue: true}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		eng.PendingOutput = ""
		eng.AwaitingName = false
		name := strings.TrimSpace(req.Input)
		if name == "" {
			name = eng.Player.Username
		}
		if err := eng.SetCharName(name); err != nil {
			http.Error(w, "erro ao salvar nome", http.StatusInternalServerError)
			return
		}
		output := stripANSI("\nBem-vindo, " + name + "!\n\n" + eng.FormatNodeOutput())
		resp := commandResponse{Output: output, Redraw: true, Continue: true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Return pending output without processing
	if eng.PendingOutput != "" {
		output := stripANSI(eng.PendingOutput)
		eng.PendingOutput = ""
		resp := commandResponse{Output: output, Redraw: true, Continue: true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	result := eng.ProcessInput(req.Input)

	// Handle character name from first command
	if result.Output == "" && eng.Player.CharName == "" {
		eng.AwaitingName = true
		eng.PendingOutput = "Digite seu nome, aventureiro: "
		result = &game.CommandResult{
			Output:      "Digite seu nome, aventureiro: ",
			NeedsRedraw: false,
			Continue:    true,
		}
	}

	// Strip ANSI for web clients
	output := stripANSI(result.Output)

	resp := commandResponse{
		Output:   output,
		Redraw:   result.NeedsRedraw,
		Continue: result.Continue,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	// If player disconnected, remove session
	if !result.Continue {
		h.session.Remove(req.Token)
	}
}

func (h *HTTPServer) handleSetSupporter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	secret := os.Getenv("SECRET_KEY")
	if secret != "" && r.Header.Get("Authorization") != "Bearer "+secret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Username string `json:"username"`
		Tier     string `json:"tier"`
		Since    string `json:"since"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	player, err := h.db.GetPlayerByUsername(req.Username)
	if err != nil {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	if err := h.db.UpdateSupporter(player.ID, req.Tier, req.Since); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *HTTPServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade: %v", err)
		return
	}
	defer conn.Close()

	// Wait for login message
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Erro: tempo de login esgotado.\n"))
		return
	}

	var login struct {
		Type     string `json:"type"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(msg, &login); err != nil || login.Type != "login" {
		conn.WriteMessage(websocket.TextMessage, []byte("Erro: envie {\"type\":\"login\",\"username\":\"x\",\"password\":\"y\"}\n"))
		return
	}

	player, err := h.db.GetPlayerByUsername(login.Username)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Erro: usuario ou senha invalidos.\n"))
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(player.PasswordHash), []byte(login.Password)) != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Erro: usuario ou senha invalidos.\n"))
		return
	}

	conn.SetReadDeadline(time.Time{})
	clear := "\x1b[2J\x1b[H"

	// Menu loop: choose world
	var chosenWorld string
	for {
		conn.WriteMessage(websocket.TextMessage, []byte(game.FormatMenu(player.World)))

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		input := strings.TrimSpace(string(msg))
		result := game.ProcessMenuChoice(input, player.World)

		switch result.Action {
		case game.MenuSelectWorld:
			chosenWorld = result.World
		case game.MenuProceed:
			chosenWorld = player.World
		case game.MenuError:
			conn.WriteMessage(websocket.TextMessage, []byte("\x1b[31m"+result.Message+"\x1b[0m\n"))
			continue
		}

		if chosenWorld != "" {
			break
		}
	}

	if chosenWorld != player.World {
		player.World = chosenWorld
		player.NodeAtual = game.GetStartNode(chosenWorld)
	}

	eng, err := game.New(h.db, player)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Erro ao carregar jogo.\n"))
		return
	}

	h.session.Set(player.ID, eng)
	defer h.session.Remove(player.ID)

	conn.WriteMessage(websocket.TextMessage, []byte(clear+eng.FormatNodeOutput()))

	// Game loop
	for {
		var msg []byte
		var err error

		if eng.IsInCombat() {
			// Combat: 15s timeout
			conn.SetReadDeadline(time.Now().Add(15 * time.Second))
			_, msg, err = conn.ReadMessage()
			if err != nil {
				// Timeout - enemy attacks
				_ = msg
				msg = []byte("__timeout__")
			}
			conn.SetReadDeadline(time.Time{})
		} else {
			_, msg, err = conn.ReadMessage()
			if err != nil {
				break
			}
		}

		input := strings.TrimSpace(string(msg))
		if input == "" && !eng.IsInCombat() && !eng.PostCombat {
			continue
		}

		result := eng.ProcessInput(input)
		output := result.Output
		if result.NeedsRedraw {
			output = clear + output
		}
		if !result.Continue {
			conn.WriteMessage(websocket.TextMessage, []byte(output))
			break
		}
		conn.WriteMessage(websocket.TextMessage, []byte(output))
	}
}

func (h *HTTPServer) handleExport(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}

	eng := h.session.Get(token)
	if eng == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	inv, _ := h.db.GetInventory(eng.Player.ID)
	export := map[string]interface{}{
		"player":    eng.Player,
		"inventory": inv,
		"node":      eng.GetNodeText(),
		"exported_at": time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=remote-save.json")
	json.NewEncoder(w).Encode(export)
}

func (h *HTTPServer) handleStoryGraph(w http.ResponseWriter, r *http.Request) {
	world := r.URL.Query().Get("world")
	if world == "" {
		world = "fantasia"
	}

	nodes, err := h.db.GetNodesByWorld(world)
	if err != nil {
		http.Error(w, "erro ao buscar nos", http.StatusInternalServerError)
		return
	}

	type graphNode struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Group string `json:"group"`
		Title string `json:"title"`
	}
	type graphEdge struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Label string `json:"label"`
	}

	var gNodes []graphNode
	var gEdges []graphEdge
	details := make(map[string]interface{})

	for _, n := range nodes {
		label := n.ID
		if len(label) > 12 {
			label = label[:12]
		}
		title := n.Texto
		if len(title) > 80 {
			title = title[:80] + "..."
		}
		gNodes = append(gNodes, graphNode{
			ID: n.ID, Label: label, Group: n.Tipo, Title: title,
		})

		// Enrich detail with choices
		type nodeDetail struct {
			ID       string         `json:"id"`
			SalaID   string         `json:"sala_id"`
			Tipo     string         `json:"tipo"`
			Texto    string         `json:"texto"`
			XP       int            `json:"xp"`
			Efeito   string         `json:"efeito"`
			Escolhas []model.Choice `json:"escolhas"`
		}
		choices, _ := h.db.GetChoices(n.ID)
		details[n.ID] = nodeDetail{
			ID: n.ID, SalaID: n.SalaID, Tipo: n.Tipo,
			Texto: n.Texto, XP: n.XP, Efeito: n.Efeito.String,
			Escolhas: choices,
		}
	}

	for _, n := range nodes {
		choices, err := h.db.GetChoices(n.ID)
		if err != nil {
			continue
		}
		for _, c := range choices {
			if c.Condicao.Valid && c.Condicao.String != "" {
				gEdges = append(gEdges, graphEdge{
					From: n.ID, To: c.NodeDestino,
					Label: fmt.Sprintf("%d [%s]", c.Ordem, c.Condicao.String),
				})
			} else {
				gEdges = append(gEdges, graphEdge{
					From: n.ID, To: c.NodeDestino,
					Label: fmt.Sprintf("%d", c.Ordem),
				})
			}
		}
	}

	resp := map[string]interface{}{
		"nodes":   gNodes,
		"edges":   gEdges,
		"details": details,
		"meta": map[string]interface{}{
			"total":      len(nodes),
			"world":      world,
			"start_node": model.StartNodes[world],
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *HTTPServer) handleSaveStory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		World string        `json:"world"`
		Nodes []model.Node  `json:"nodes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.World == "" {
		req.World = "fantasia"
	}

	// Remove existing nodes for this world then re-insert
	for _, n := range req.Nodes {
		// Delete old choices first
		h.db.GetChoices(n.ID) // ignore
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Salvo (funcionalidade parcial)"})
}

func (h *HTTPServer) handleSupporter(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}

	player, err := h.db.GetPlayerByUsername(username)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"username": player.Username,
		"tier":     player.SupporterTier,
		"since":    player.SupporterSince,
	})
}
