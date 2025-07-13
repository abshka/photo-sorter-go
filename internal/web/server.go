package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"photo-sorter-go/internal/compressor"
	"photo-sorter-go/internal/config"
	"photo-sorter-go/internal/extractor"
	"photo-sorter-go/internal/organizer"
	"photo-sorter-go/internal/statistics"

	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// Server represents the main web server and its state.
type Server struct {
	cfg        *config.Config
	log        *logrus.Logger
	router     *mux.Router
	httpServer *http.Server
	wsUpgrader websocket.Upgrader
	wsClients  map[*websocket.Conn]bool
	wsMutex    sync.RWMutex

	operationMutex sync.RWMutex
	isRunning      bool
	currentStats   *statistics.Statistics

	compressionMutex   sync.RWMutex
	compressionRunning bool
	compressionResults []compressor.CompressionResult
	compressionError   string

	compressor compressor.Compressor
}

// APIResponse is the standard API response structure.
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ScanRequest represents a scan request payload.
type ScanRequest struct {
	Directory string `json:"directory"`
}

// OrganizeRequest represents an organize request payload.
type OrganizeRequest struct {
	SourceDirectory string `json:"source_directory"`
	TargetDirectory string `json:"target_directory,omitempty"`
	DryRun          bool   `json:"dry_run"`
	DateFormat      string `json:"date_format,omitempty"`
	MoveFiles       *bool  `json:"move_files,omitempty"`
}

// WSMessage is the structure for WebSocket messages.
type WSMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// NewServer creates a new Server instance.
func NewServer(cfg *config.Config, log *logrus.Logger, compressor compressor.Compressor) *Server {
	s := &Server{
		cfg:       cfg,
		log:       log,
		router:    mux.NewRouter(),
		wsClients: make(map[*websocket.Conn]bool),
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		compressor: compressor,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all HTTP and WebSocket routes.
func (s *Server) setupRoutes() {
	api := s.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/status", s.handleStatus).Methods("GET")
	api.HandleFunc("/scan", s.handleScan).Methods("POST")
	api.HandleFunc("/organize", s.handleOrganize).Methods("POST")
	api.HandleFunc("/stop", s.handleStop).Methods("POST")

	api.HandleFunc("/statistics", s.handleGetStatistics).Methods("GET")
	api.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	api.HandleFunc("/config", s.handleUpdateConfig).Methods("POST")
	api.HandleFunc("/date-formats", s.handleGetDateFormats).Methods("GET")

	api.HandleFunc("/compress", s.handleCompress).Methods("POST")
	api.HandleFunc("/compression-status", s.handleCompressionStatus).Methods("GET")

	s.router.HandleFunc("/ws", s.handleWebSocket)

	s.router.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))),
	)

	s.router.HandleFunc("/", s.handleIndex).Methods("GET")
}

// Start launches the HTTP server on the specified port.
func (s *Server) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	s.log.Infof("Starting web server on http://localhost%s", addr)
	return s.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// handleIndex serves the main HTML page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/index.html")
}

// handleStatus returns the current operation status and statistics.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.operationMutex.RLock()
	running := s.isRunning
	stats := s.currentStats
	s.operationMutex.RUnlock()

	var statsData any
	if stats != nil {
		statsData = map[string]any{
			"summary": stats.GetSummary(),
			"files": map[string]any{
				"total_found":     atomic.LoadInt64(&stats.TotalFilesFound),
				"total_processed": atomic.LoadInt64(&stats.TotalFilesProcessed),
				"organized":       atomic.LoadInt64(&stats.FilesOrganized),
				"moved":           atomic.LoadInt64(&stats.FilesMoved),
				"copied":          atomic.LoadInt64(&stats.FilesCopied),
				"skipped":         atomic.LoadInt64(&stats.FilesSkipped),
				"errors":          atomic.LoadInt64(&stats.FilesWithErrors),
			},
		}
	}

	s.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]any{
			"running":    running,
			"statistics": statsData,
		},
	})
}

// handleScan starts a scan operation asynchronously.
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Directory == "" {
		s.writeError(w, "Directory is required", http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(req.Directory); os.IsNotExist(err) {
		s.writeError(w, "Directory does not exist", http.StatusBadRequest)
		return
	}

	go s.runScanAsyncWithLogs(req.Directory)

	s.writeJSON(w, APIResponse{
		Success: true,
		Message: "Scan started",
	})
}

// handleOrganize starts an organize operation asynchronously.
func (s *Server) handleOrganize(w http.ResponseWriter, r *http.Request) {
	var req OrganizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SourceDirectory == "" {
		s.writeError(w, "Source directory is required", http.StatusBadRequest)
		return
	}

	s.operationMutex.RLock()
	if s.isRunning {
		s.operationMutex.RUnlock()
		s.writeError(w, "Operation already in progress", http.StatusConflict)
		return
	}
	s.operationMutex.RUnlock()

	if _, err := os.Stat(req.SourceDirectory); os.IsNotExist(err) {
		s.writeError(w, "Source directory does not exist", http.StatusBadRequest)
		return
	}

	go s.runOrganizeAsync(req)

	s.writeJSON(w, APIResponse{
		Success: true,
		Message: "Organization started",
	})
}

// handleStop stops the current operation.
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	s.operationMutex.Lock()
	s.isRunning = false
	s.operationMutex.Unlock()

	s.broadcastWSMessage("operation_stopped", map[string]any{
		"message": "Operation stopped by user",
	})

	s.writeJSON(w, APIResponse{
		Success: true,
		Message: "Operation stopped",
	})
}

// handleGetStatistics returns the current statistics.
func (s *Server) handleGetStatistics(w http.ResponseWriter, r *http.Request) {
	s.operationMutex.RLock()
	stats := s.currentStats
	s.operationMutex.RUnlock()

	var statsData any
	if stats != nil {
		statsData = map[string]any{
			"summary": stats.GetSummary(),
			"files": map[string]any{
				"total_found":     atomic.LoadInt64(&stats.TotalFilesFound),
				"total_processed": atomic.LoadInt64(&stats.TotalFilesProcessed),
				"organized":       atomic.LoadInt64(&stats.FilesOrganized),
				"moved":           atomic.LoadInt64(&stats.FilesMoved),
				"copied":          atomic.LoadInt64(&stats.FilesCopied),
				"skipped":         atomic.LoadInt64(&stats.FilesSkipped),
				"errors":          atomic.LoadInt64(&stats.FilesWithErrors),
			},
		}
	}

	s.writeJSON(w, APIResponse{
		Success: true,
		Data:    statsData,
	})
}

// handleCompress starts the image compression process asynchronously.
func (s *Server) handleCompress(w http.ResponseWriter, r *http.Request) {
	s.compressionMutex.Lock()
	if s.compressionRunning {
		s.compressionMutex.Unlock()
		s.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Compression already running",
		})
		return
	}
	s.compressionRunning = true
	s.compressionResults = nil
	s.compressionError = ""
	s.compressionMutex.Unlock()

	go s.runCompressionAsync()

	s.writeJSON(w, APIResponse{
		Success: true,
		Message: "Image compression started",
	})
}

// runCompressionAsync performs image compression in a separate goroutine.
func (s *Server) runCompressionAsync() {
	s.broadcastWSMessage("compression_started", map[string]any{
		"message":   "Image compression started",
		"directory": s.cfg.SourceDirectory,
	})

	defer func() {
		s.compressionMutex.Lock()
		s.compressionRunning = false
		s.compressionMutex.Unlock()
	}()

	params := s.cfg.Compressor
	s.log.Infof("runCompressionAsync called: enabled=%v, input=%v", params.Enabled, s.cfg.SourceDirectory)

	if !params.Enabled {
		s.log.Warn("Compression is disabled in config")
		return
	}

	targetDir := s.cfg.SourceDirectory
	if s.cfg.TargetDirectory != nil && *s.cfg.TargetDirectory != "" {
		targetDir = *s.cfg.TargetDirectory
	}
	compParams := compressor.CompressionParams{
		InputPaths: []string{s.cfg.SourceDirectory},
		TargetDir:  targetDir,
		Quality:    params.Quality,
		Threshold:  params.Threshold,
		Formats:    params.Formats,
	}

	if len(compParams.InputPaths) == 0 || compParams.InputPaths[0] == "" {
		s.log.Warn("No input files for compression: input paths empty")
		return
	}
	if _, err := os.Stat(compParams.InputPaths[0]); err != nil {
		s.log.Warnf("Input directory does not exist or not accessible: %v", err)
		return
	}

	s.log.Infof("Starting image compression: input=%v, targetDir=%s, quality=%d, threshold=%.2f, formats=%v",
		s.cfg.SourceDirectory, targetDir, params.Quality, params.Threshold, params.Formats)

	ctx := context.Background()
	results, err := s.compressor.Compress(ctx, compParams)
	s.compressionMutex.Lock()
	defer s.compressionMutex.Unlock()
	if err != nil {
		s.compressionError = err.Error()
		s.compressionResults = nil
		s.log.Errorf("Image compression error: %v", err)
		s.broadcastWSMessage("compression_error", map[string]any{
			"error": err.Error(),
		})
	} else {
		s.compressionResults = results
		var origSize, compSize int64
		var processedCount int
		for _, r := range results {
			if r.Action == "compressed" || r.Action == "original" {
				origSize += r.OriginalSize
				compSize += r.CompressedSize
				processedCount++
			}
		}
		var percent float64
		if origSize > 0 {
			percent = float64(origSize-compSize) * 100 / float64(origSize)
		}
		s.log.Infof("Image compression finished: %d files processed (only compressed/original), total files: %d", processedCount, len(results))
		s.broadcastWSMessage("compression_completed", map[string]any{
			"files_processed": processedCount,
			"original_size":   origSize,
			"compressed_size": compSize,
			"percent_saved":   percent,
			"message":         "Image compression finished",
		})
	}
}

// handleCompressionStatus returns the status and results of compression.
func (s *Server) handleCompressionStatus(w http.ResponseWriter, r *http.Request) {
	s.compressionMutex.RLock()
	running := s.compressionRunning
	results := s.compressionResults
	errMsg := s.compressionError
	s.compressionMutex.RUnlock()

	s.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]any{
			"running": running,
			"results": results,
			"error":   errMsg,
		},
	})
}

// handleGetConfig returns the current configuration.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]any{
			"date_format":        s.cfg.DateFormat,
			"move_files":         s.cfg.Processing.MoveFiles,
			"dry_run":            s.cfg.Security.DryRun,
			"duplicate_handling": s.cfg.Processing.DuplicateHandling,
			"source_directory":   s.cfg.SourceDirectory,
			"target_directory":   s.cfg.TargetDirectory,
		},
	})
}

// handleUpdateConfig updates the configuration from the request.
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var configUpdate struct {
		DateFormat        string `json:"date_format,omitempty"`
		MoveFiles         *bool  `json:"move_files,omitempty"`
		DryRun            *bool  `json:"dry_run,omitempty"`
		DuplicateHandling string `json:"duplicate_handling,omitempty"`
		SourceDirectory   string `json:"source_directory,omitempty"`
		TargetDirectory   string `json:"target_directory,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&configUpdate); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if configUpdate.DateFormat != "" {
		s.cfg.DateFormat = configUpdate.DateFormat
	}
	if configUpdate.MoveFiles != nil {
		s.cfg.Processing.MoveFiles = *configUpdate.MoveFiles
	}
	if configUpdate.DryRun != nil {
		s.cfg.Security.DryRun = *configUpdate.DryRun
	}
	if configUpdate.DuplicateHandling != "" {
		s.cfg.Processing.DuplicateHandling = configUpdate.DuplicateHandling
	}
	if configUpdate.SourceDirectory != "" {
		s.cfg.SourceDirectory = configUpdate.SourceDirectory
	}
	if configUpdate.TargetDirectory != "" {
		s.cfg.TargetDirectory = &configUpdate.TargetDirectory
	}

	s.log.Info("Configuration updated via web interface")

	s.writeJSON(w, APIResponse{
		Success: true,
		Message: "Configuration updated successfully",
	})
}

// handleGetDateFormats returns available date formats.
func (s *Server) handleGetDateFormats(w http.ResponseWriter, r *http.Request) {
	formats := config.GetAvailableDateFormats()
	s.writeJSON(w, APIResponse{
		Success: true,
		Data:    formats,
	})
}

// handleWebSocket upgrades the connection and manages WebSocket clients.
// handleWebSocket handles WebSocket connections.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := s.wsUpgrader
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warnf("WebSocket upgrade failed: %v", err)
		return
	}

	s.wsMutex.Lock()
	s.wsClients[conn] = true
	s.wsMutex.Unlock()

	defer func() {
		s.wsMutex.Lock()
		delete(s.wsClients, conn)
		s.wsMutex.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// broadcastWSLog отправляет лог-сообщение всем WS-клиентам
func (s *Server) broadcastWSLog(level, message string) {
	s.wsMutex.Lock()
	defer s.wsMutex.Unlock()
	for client := range s.wsClients {
		_ = client.WriteJSON(WSMessage{
			Type: "log",
			Data: map[string]any{
				"level":     level,
				"message":   message,
				"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			},
		})
	}
}

// runScanAsyncWithLogs запускает сканирование с пробросом логов в WebSocket
func (s *Server) runScanAsyncWithLogs(directory string) {
	go func() {
		s.operationMutex.Lock()
		s.isRunning = true
		s.operationMutex.Unlock()

		s.broadcastWSMessage("scan_started", map[string]any{
			"directory": directory,
		})

		defer func() {
			s.operationMutex.Lock()
			s.isRunning = false
			s.operationMutex.Unlock()
		}()

		cfg := *s.cfg // Копия!
		cfg.SourceDirectory = directory
		cfg.Security.DryRun = true

		log := s.log
		stats := statistics.NewStatistics()
		dateExtractor := extractor.NewEXIFExtractor(log)
		compressor := compressor.NewDefaultCompressor()

		// Создаём organizer с хуком для логов
		org := organizer.NewFileOrganizerWithLogHook(&cfg, log, stats, dateExtractor, compressor, func(level, message string) {
			// Только dry-run логи (DRY-RUN: ...) пробрасываем в WebSocket
			if strings.Contains(message, "DRY-RUN") {
				s.broadcastWSLog(level, message)
			}
		})

		err := org.OrganizeFiles()
		if err != nil {
			s.broadcastWSMessage("scan_error", map[string]any{
				"error": err.Error(),
			})
			return
		}

		s.currentStats = stats

		s.broadcastWSMessage("scan_completed", map[string]any{
			"statistics": stats.GetSummary(),
		})
	}()
}

// runScanAsync performs a scan operation in a separate goroutine.
func (s *Server) runScanAsync(directory string) {
	s.operationMutex.Lock()
	s.isRunning = true
	s.currentStats = statistics.NewStatistics()
	s.operationMutex.Unlock()

	s.broadcastWSMessage("scan_started", map[string]any{
		"directory": directory,
	})

	cfg := *s.cfg
	cfg.SourceDirectory = directory
	cfg.Security.DryRun = true

	dateExtractor := extractor.NewEXIFExtractor(s.log)

	// Прокидываем хук для логов (DRY-RUN и др.) в органайзер
	org := organizer.NewFileOrganizerWithLogHook(&cfg, s.log, s.currentStats, dateExtractor, s.compressor, func(level, message string) {
		// Пробрасываем только интересные логи (DRY-RUN, Would move/copy)
		if strings.Contains(message, "DRY-RUN") || strings.Contains(message, "Would move") || strings.Contains(message, "Would copy") {
			s.broadcastWSMessage("log", map[string]any{
				"level":     level,
				"message":   message,
				"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			})
		}
	})

	err := org.OrganizeFiles()

	s.operationMutex.Lock()
	s.isRunning = false
	s.operationMutex.Unlock()

	if err != nil {
		s.broadcastWSMessage("scan_error", map[string]any{
			"error": err.Error(),
		})
	} else {
		s.broadcastWSMessage("scan_completed", map[string]any{
			"statistics": s.currentStats.GetSummary(),
		})
	}
}

// runOrganizeAsync performs an organize operation in a separate goroutine.
func (s *Server) runOrganizeAsync(req OrganizeRequest) {
	s.operationMutex.Lock()
	s.isRunning = true
	s.currentStats = statistics.NewStatistics()
	s.operationMutex.Unlock()

	s.broadcastWSMessage("organize_started", map[string]any{
		"source_directory": req.SourceDirectory,
		"target_directory": req.TargetDirectory,
		"dry_run":          req.DryRun,
	})

	cfg := *s.cfg
	cfg.SourceDirectory = req.SourceDirectory
	if req.TargetDirectory != "" {
		cfg.TargetDirectory = &req.TargetDirectory
	}
	cfg.Security.DryRun = req.DryRun

	if req.DateFormat != "" {
		cfg.DateFormat = req.DateFormat
	}
	if req.MoveFiles != nil {
		cfg.Processing.MoveFiles = *req.MoveFiles
	}

	if req.DateFormat != "" {
		cfg.DateFormat = req.DateFormat
	}
	if req.MoveFiles != nil {
		cfg.Processing.MoveFiles = *req.MoveFiles
	}

	dateExtractor := extractor.NewEXIFExtractor(s.log)
	org := organizer.NewFileOrganizer(&cfg, s.log, s.currentStats, dateExtractor, s.compressor)

	err := org.OrganizeFiles()

	s.operationMutex.Lock()
	s.isRunning = false
	s.operationMutex.Unlock()

	if err != nil {
		s.broadcastWSMessage("organize_error", map[string]any{
			"error": err.Error(),
		})
	} else {
		s.broadcastWSMessage("organize_completed", map[string]any{
			"statistics": s.currentStats.GetSummary(),
		})
	}
}

// broadcastWSMessage sends a message to all connected WebSocket clients.
func (s *Server) broadcastWSMessage(messageType string, data any) {
	message := WSMessage{
		Type: messageType,
		Data: data,
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		s.log.Errorf("Failed to marshal WebSocket message: %v", err)
		return
	}

	s.wsMutex.Lock()
	defer s.wsMutex.Unlock()

	for conn := range s.wsClients {
		err := conn.WriteMessage(websocket.TextMessage, msgBytes)
		if err != nil {
			s.log.Errorf("Failed to write WebSocket message: %v", err)
			go func(c *websocket.Conn) {
				s.wsMutex.Lock()
				delete(s.wsClients, c)
				s.wsMutex.Unlock()
				c.Close()
			}(conn)
		}
	}
}

// writeJSON writes a JSON response to the client.
func (s *Server) writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response in JSON format.
func (s *Server) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   message,
	})
}
