package edr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ethereum-optimism/supersim/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// EDR represents the Ethereum Debugging Runtime
type EDR struct {
	logFilePath string

	cfg *config.ChainConfig

	closeApp context.CancelCauseFunc
}

// TraceResult represents the result of a trace operation
type TraceResult struct {
	Type         string          `json:"type"`
	Action       json.RawMessage `json:"action"`
	Result       json.RawMessage `json:"result"`
	SubTraces    []TraceResult   `json:"subtraces"`
	TraceAddress []int           `json:"traceAddress"`
	Error        string          `json:"error,omitempty"`
}

// New creates a new EDR instance
func New(log log.Logger, closeApp context.CancelCauseFunc, cfg *config.ChainConfig) *EDR {
	return &EDR{
		cfg:      cfg,
		closeApp: closeApp,
	}
}

// Start initializes and starts the EDR service
func (e *EDR) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Register handlers
	mux.HandleFunc("/trace", e.handleTrace)
	mux.HandleFunc("/debug", e.handleDebug)
	mux.HandleFunc("/hardhat", e.handleHardhat)

	e.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", e.cfg.Port),
		Handler: mux,
	}

	go func() {
		e.log.Info("Starting EDR server", "port", e.cfg.Port)
		if err := e.server.ListenAndServe(); err != http.ErrServerClosed {
			e.log.Error("EDR server error", "err", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the EDR service
func (e *EDR) Stop(ctx context.Context) error {
	if e.stopped.Load() {
		return fmt.Errorf("EDR already stopped")
	}
	defer e.stopped.Store(true)

	if e.server != nil {
		e.log.Info("Stopping EDR server")
		return e.server.Shutdown(ctx)
	}
	return nil
}

// TraceTransaction traces a specific transaction
func (e *EDR) TraceTransaction(ctx context.Context, hash common.Hash) (*TraceResult, error) {
	if !e.cfg.TracerEnabled {
		return nil, fmt.Errorf("tracing not enabled")
	}

	return e.chain.TraceTransaction(ctx, hash)
}

// Debug provides debugging information for a transaction or block
func (e *EDR) Debug(ctx context.Context, data interface{}) (*TraceResult, error) {
	// Implementation will depend on EDR spec
	return nil, fmt.Errorf("not implemented")
}

// HTTP Handlers

func (e *EDR) handleTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TxHash string `json:"txHash"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hash := common.HexToHash(req.TxHash)
	result, err := e.TraceTransaction(r.Context(), hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

// Hardhat compatibility methods

func (e *EDR) SetNextBlockTimestamp(ctx context.Context, timestamp uint64) error {
	if !e.cfg.HardhatMode {
		return fmt.Errorf("hardhat mode not enabled")
	}
	return e.chain.SetNextBlockTimestamp(ctx, timestamp)
}

func (e *EDR) Mine(ctx context.Context, blocks uint64) error {
	if !e.cfg.HardhatMode {
		return fmt.Errorf("hardhat mode not enabled")
	}
	return e.chain.Mine(ctx, blocks)
}

// Helper methods

func (e *EDR) Endpoint() string {
	return fmt.Sprintf("http://localhost:%d", e.cfg.Port)
}

func (e *EDR) LogPath() string {
	return e.logFilePath
}

// Config returns the chain configuration
func (e *EDR) Config() *config.ChainConfig {
	return e.chain.Config()
}

// EthClient returns the ethereum client
func (e *EDR) EthClient() *ethclient.Client {
	return e.chain.EthClient()
}
