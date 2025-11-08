package api

import (
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"strings"

	"dex-aggregator/config"
	"dex-aggregator/internal/aggregator"
	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/mux"
)

type Handler struct {
	router *aggregator.Router
	cache  cache.Store
}

func NewHandler(router *aggregator.Router, cache cache.Store) *Handler {
	return &Handler{
		router: router,
		cache:  cache,
	}
}

func (h *Handler) GetQuote(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		log.Printf("Invalid content type: %s", contentType)
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	var req types.QuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode JSON: %v", err)
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Quote request: %s -> %s, amount: %s", req.TokenIn, req.TokenOut, req.AmountIn.String())

	if req.TokenIn == "" || req.TokenOut == "" {
		http.Error(w, "tokenIn and tokenOut are required", http.StatusBadRequest)
		return
	}

	if !common.IsHexAddress(req.TokenIn) {
		http.Error(w, "Invalid tokenIn address", http.StatusBadRequest)
		return
	}

	if !common.IsHexAddress(req.TokenOut) {
		http.Error(w, "Invalid tokenOut address", http.StatusBadRequest)
		return
	}

	if req.AmountIn == nil || req.AmountIn.Cmp(big.NewInt(0)) <= 0 {
		http.Error(w, "Invalid input amount", http.StatusBadRequest)
		return
	}

	if req.MaxHops == 0 {
		req.MaxHops = 3
	}

	resp, err := h.router.GetBestQuote(r.Context(), &req)
	if err != nil {
		log.Printf("Quote calculation failed: %v", err)
		http.Error(w, "Quote calculation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Quote successful: %s -> %s", req.AmountIn.String(), resp.AmountOut.String())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

func (h *Handler) GetPools(w http.ResponseWriter, r *http.Request) {
	pools, err := h.cache.GetAllPools(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch pools: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if pools == nil {
		pools = []*types.Pool{}
	}

	response := map[string]interface{}{
		"count": len(pools),
		"pools": pools,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) GetPoolByAddress(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	if address == "" {
		http.Error(w, "Pool address is required", http.StatusBadRequest)
		return
	}

	log.Printf("Looking for pool by address: %s", address)

	pool, err := h.cache.GetPool(r.Context(), address)
	if err != nil {
		log.Printf("Pool not found: %s", address)
		http.Error(w, "Pool not found: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
}

func (h *Handler) GetPoolsByTokens(w http.ResponseWriter, r *http.Request) {
	tokenA := r.URL.Query().Get("tokenA")
	tokenB := r.URL.Query().Get("tokenB")

	if tokenA == "" || tokenB == "" {
		http.Error(w, "Both tokenA and tokenB parameters are required", http.StatusBadRequest)
		return
	}

	normalizedTokenA := strings.ToLower(tokenA)
	normalizedTokenB := strings.ToLower(tokenB)

	log.Printf("API: Searching pools for token pair: %s / %s", tokenA, tokenB)

	pools, err := h.cache.GetPoolsByTokens(r.Context(), tokenA, tokenB)
	if err != nil {
		log.Printf("API: Error fetching pools: %v", err)
		http.Error(w, "Failed to fetch pools: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if pools == nil {
		pools = []*types.Pool{}
	}

	log.Printf("API: Successfully found %d pools", len(pools))

	response := map[string]interface{}{
		"tokenA":      tokenA,
		"tokenB":      tokenB,
		"normalizedA": normalizedTokenA,
		"normalizedB": normalizedTokenB,
		"count":       len(pools),
		"pools":       pools,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	configInfo := map[string]interface{}{
		"server": map[string]interface{}{
			"port":          config.AppConfig.Server.Port,
			"read_timeout":  config.AppConfig.Server.ReadTimeout,
			"write_timeout": config.AppConfig.Server.WriteTimeout,
		},
		"redis": map[string]interface{}{
			"addr": config.AppConfig.Redis.Addr,
			"db":   config.AppConfig.Redis.DB,
		},
		"ethereum": map[string]interface{}{
			"rpc_url":  config.AppConfig.Ethereum.RPCURL,
			"chain_id": config.AppConfig.Ethereum.ChainID,
		},
		"dex": map[string]interface{}{
			"base_tokens": config.AppConfig.DEX.BaseTokens,
			"token_count": len(config.AppConfig.DEX.BaseTokens),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configInfo)
}
