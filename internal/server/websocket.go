package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/creack/pty/v2"
	"github.com/gorilla/websocket"
)

// Protocol message types (matching GoTTY protocol).
const (
	msgInput          = '1'
	msgResizeTerminal = '3'
	msgOutput         = '1'
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from same origin
		return true
	},
}

// resizeMessage represents a resize message from the terminal client.
type resizeMessage struct {
	Columns uint16 `json:"columns"`
	Rows    uint16 `json:"rows"`
}

// handleTerminalWebSocket handles WebSocket connections for terminal sessions.
func (s *Server) handleTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Get PTY for the session
	ptmx, err := s.sessionManager.GetPTY(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		slog.Error("failed to get PTY for session", "session_id", sessionID, "error", err)
		return
	}

	// Check if there's an existing connection for this session
	s.connectionsMu.Lock()
	if existingConn, exists := s.activeConnections[sessionID]; exists {
		slog.Info("closing existing WebSocket connection", "session_id", sessionID)
		// Cancel existing goroutines
		if cancel, ok := s.cancelFuncs[sessionID]; ok {
			cancel()
			delete(s.cancelFuncs, sessionID)
		}
		_ = existingConn.Close() // Best effort close
		delete(s.activeConnections, sessionID)
	}
	s.connectionsMu.Unlock()

	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("failed to upgrade WebSocket connection", "error", err)
		return
	}

	// Create context for this connection
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		_ = conn.Close() // Best effort close
		// Remove from active connections
		s.connectionsMu.Lock()
		delete(s.activeConnections, sessionID)
		delete(s.cancelFuncs, sessionID)
		s.connectionsMu.Unlock()
	}()

	// Register new connection
	s.connectionsMu.Lock()
	s.activeConnections[sessionID] = conn
	s.cancelFuncs[sessionID] = cancel
	s.connectionsMu.Unlock()

	slog.Info("WebSocket connected", "session_id", sessionID)

	// Setup bidirectional communication
	var wg sync.WaitGroup
	wg.Add(2)

	// PTY -> WebSocket (read from PTY, send to browser)
	go func() {
		defer wg.Done()

		// Create a channel for PTY data
		type ptyData struct {
			data []byte
			err  error
		}
		dataChan := make(chan ptyData, 1)

		// Start PTY reader goroutine
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := ptmx.Read(buf)
				if err != nil {
					dataChan <- ptyData{nil, err}
					return
				}
				if n > 0 {
					// Copy data to avoid race condition
					data := make([]byte, n)
					copy(data, buf[:n])
					dataChan <- ptyData{data, nil}
				}
			}
		}()

		// Main loop with context cancellation
		for {
			select {
			case <-ctx.Done():
				return
			case result := <-dataChan:
				if result.err != nil {
					if result.err != io.EOF {
						slog.Error("failed to read from PTY", "error", result.err)
					}
					return
				}
				if len(result.data) > 0 {
					// Encode data as base64 and prefix with message type (GoTTY protocol)
					encoded := base64.StdEncoding.EncodeToString(result.data)
					message := string(msgOutput) + encoded
					if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
						slog.Error("failed to write to WebSocket", "error", err)
						return
					}
				}
			}
		}
	}()

	// WebSocket -> PTY (read from browser, send to PTY)
	go func() {
		defer wg.Done()
		for {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			msgType, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					slog.Error("WebSocket error", "error", err)
				}
				return
			}
			if len(data) > 0 {
				// Parse protocol message (GoTTY style)
				if msgType == websocket.TextMessage && len(data) > 0 {
					msgTypeChar := data[0]
					payload := data[1:]

					switch msgTypeChar {
					case msgInput:
						// Decode base64 input data
						decoded, err := base64.StdEncoding.DecodeString(string(payload))
						if err != nil {
							slog.Error("failed to decode input", "error", err)
							continue
						}
						// Write decoded data to PTY
						if _, err := ptmx.Write(decoded); err != nil {
							slog.Error("failed to write to PTY", "error", err)
							return
						}

					case msgResizeTerminal:
						// Handle resize message
						var msg resizeMessage
						if err := json.Unmarshal(payload, &msg); err != nil {
							slog.Error("failed to parse resize message", "error", err)
							continue
						}
						winSize := pty.Winsize{
							Rows: msg.Rows,
							Cols: msg.Columns,
						}
						if err := pty.Setsize(ptmx, &winSize); err != nil {
							slog.Error("failed to resize PTY", "error", err)
						} else {
							slog.Debug("resized PTY", "columns", msg.Columns, "rows", msg.Rows, "session_id", sessionID)
						}

					default:
						slog.Warn("unknown message type", "type", string(msgTypeChar))
					}
				}
			}
		}
	}()

	wg.Wait()
	slog.Info("WebSocket disconnected", "session_id", sessionID)
}
