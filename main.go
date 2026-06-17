package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

var (
	// Configuration
	UPSTREAM_RPC               = "https://eth.rpc.bloxroute.com"
	FLASHBOTS_RELAY            = "https://relay.flashbots.net"
	PORT                       = ":8545"
	FEE_PERCENTAGE             = 0.10 // 10% optimization fee
	BACKRUN_SIGNER_PK          = ""   // Your private key for signing backruns
	FEE_RECIPIENT              = ""   // Your wallet address for fee collection
	CLIENT_HEADER_NAME         = "X-StarkGateway-Client"
	TRUSTED_CLIENTS            = "" // comma-separated trusted client identifiers
	FORCE_PRIVATE_SEND_TX      = false
	FORCE_PRIVATE_SEND_TX_KEY  = "X-StarkGateway-Force-Private"
	ENABLE_SANDWICH_PROTECTION = true
	MAX_BUNDLE_AGE_BLOCKS      = 2

	// Builder & OFA Relays (multi-route for maximum MEV capture)
	BUILDER_RELAYS = []string{
		"https://relay.flashbots.net",    // Flashbots MEV-Boost
		"https://rpc.mevboost.builders/", // MEV-Boost Builders
		"https://eth-builder.xyz/bundle", // EigenLayer AVS
	}

	// Ethereum Constants
	MAINNET_ID = int64(1)
	UNISWAP_V3 = common.HexToAddress("0xE592427A0AEce92De3Edee1F18E0157C05861564")
	UNISWAP_V2 = common.HexToAddress("0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D")
	WETH       = common.HexToAddress("0xC02aaA39b223FE8D0A0e8e4F27ead9083C756Cc2")
	SUSHISWAP  = common.HexToAddress("0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F")
	CURVE_POOL = common.HexToAddress("0x0000000000000000000000000000000000000000") // placeholder for curve router if needed

	// Metrics
	metrics = struct {
		sync.Mutex
		totalFees              float64
		txProcessed            int64
		trustedRequests        int64
		mevDetected            int64
		backrunsExec           int64
		flashbotsBundlesSent   int64
		flashbotsBundlesOK     int64
		flashbotsFallbacks     int64
		bundlesFailed          int64
		sandwichAttacksBlocked int64
		multiBuilderSent       int64
		clientCounts           map[string]int64
	}{
		clientCounts: make(map[string]int64),
	}

	// Backrun signer key
	backrunKey  *ecdsa.PrivateKey
	backrunAddr common.Address
)

var trustedClients = map[string]bool{}

var httpClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        200,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConnsPerHost: 100,
	},
	Timeout: 12 * time.Second,
}

// JSONRPCRequest represents a standard JSON-RPC 2.0 request
type JSONRPCRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

// JSONRPCResponse represents a standard JSON-RPC 2.0 response
type JSONRPCResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// Transaction wraps parsed tx data
type Transaction struct {
	From     common.Address
	To       *common.Address
	Value    string
	Data     string
	GasPrice string
	Gas      uint64
	Nonce    uint64
}

func init() {
	godotenv.Load()

	if rpc := os.Getenv("UPSTREAM_RPC"); rpc != "" {
		UPSTREAM_RPC = rpc
	}
	if relay := os.Getenv("FLASHBOTS_RELAY"); relay != "" {
		FLASHBOTS_RELAY = relay
	}
	if port := os.Getenv("PORT"); port != "" {
		PORT = ":" + port
	}
	if fee := os.Getenv("FEE_PERCENTAGE"); fee != "" {
		fmt.Sscanf(fee, "%f", &FEE_PERCENTAGE)
	}
	if header := os.Getenv("CLIENT_HEADER_NAME"); header != "" {
		CLIENT_HEADER_NAME = header
	}

	// Load or generate backrun signer private key
	if pk := os.Getenv("BACKRUN_SIGNER_PK"); pk != "" {
		BACKRUN_SIGNER_PK = pk
	} else {
		// Generate random key if not provided
		key, _ := crypto.GenerateKey()
		BACKRUN_SIGNER_PK = hexutil.Encode(crypto.FromECDSA(key))
		log.Printf("⚠️  Generated ephemeral backrun key: %s", BACKRUN_SIGNER_PK[:10]+"...")
		log.Printf("💾 Save this in .env for persistence: BACKRUN_SIGNER_PK=%s", BACKRUN_SIGNER_PK)
	}

	// Parse backrun signer
	pk := BACKRUN_SIGNER_PK
	if len(pk) > 2 && pk[:2] == "0x" {
		pk = pk[2:]
	}
	key, err := crypto.HexToECDSA(pk)
	if err != nil {
		log.Fatalf("❌ Failed to parse backrun private key: %v", err)
	}
	backrunKey = key
	backrunAddr = crypto.PubkeyToAddress(key.PublicKey)

	if fee := os.Getenv("FEE_RECIPIENT"); fee != "" {
		FEE_RECIPIENT = fee
	} else {
		FEE_RECIPIENT = backrunAddr.Hex()
		log.Printf("📍 Fee recipient (backrun signer): %s", FEE_RECIPIENT)
	}

	if list := os.Getenv("TRUSTED_CLIENTS"); list != "" {
		for _, item := range strings.Split(list, ",") {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				trustedClients[trimmed] = true
			}
		}
		log.Printf("🔐 Trusted clients loaded: %d", len(trustedClients))
	}

	if env := os.Getenv("FORCE_PRIVATE_SEND_TX"); env != "" {
		if strings.EqualFold(env, "true") || env == "1" {
			FORCE_PRIVATE_SEND_TX = true
		}
	}

	log.Printf("⚡ Flashbots Relay: %s", FLASHBOTS_RELAY)
	log.Printf("🔒 Force private eth_sendTransaction: %v", FORCE_PRIVATE_SEND_TX)
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/", handleRPC).Methods("POST")
	router.HandleFunc("/health", handleHealth).Methods("GET")
	router.HandleFunc("/metrics", handleMetrics).Methods("GET")

	log.Printf("🚀 StarkGateway starting on %s", PORT)
	log.Printf("📡 Upstream RPC: %s", UPSTREAM_RPC)
	log.Fatal(http.ListenAndServe(PORT, router))
}

// handleRPC is the main RPC proxy handler
func handleRPC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	clientID := strings.TrimSpace(r.Header.Get(CLIENT_HEADER_NAME))
	metrics.Lock()
	metrics.txProcessed++
	if clientID != "" {
		metrics.clientCounts[clientID]++
	}
	metrics.Unlock()

	trusted := isTrustedClient(r)
	if trusted {
		metrics.Lock()
		metrics.trustedRequests++
		metrics.Unlock()
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, nil, "Invalid request", -32700)
		return
	}
	defer r.Body.Close()

	// Parse JSON-RPC request
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respondError(w, nil, "Parse error", -32700)
		return
	}

	// Route based on method
	switch req.Method {
	case "eth_sendRawTransaction":
		handleSendRawTransaction(w, req, body, trusted)
	case "eth_sendTransaction":
		handleSendTransaction(w, req, body, trusted, r)
	default:
		// Pass through to upstream RPC for all other methods
		proxyRequest(w, req, body, r)
	}
}

// handleSendRawTransaction intercepts raw transactions for MEV analysis
func handleSendRawTransaction(w http.ResponseWriter, req JSONRPCRequest, body []byte, trusted bool) {
	// Extract the raw transaction hex
	var params []string
	if err := json.Unmarshal(req.Params, &params); err != nil || len(params) == 0 {
		respondError(w, req.ID, "Invalid params", -32602)
		return
	}

	txHex := params[0]

	// Decode transaction
	txBytes, err := hexutil.Decode(txHex)
	if err != nil {
		respondError(w, req.ID, "Invalid transaction", -32602)
		return
	}

	// Parse transaction
	tx := &types.Transaction{}
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		respondError(w, req.ID, "Invalid transaction", -32602)
		return
	}

	// Analyze for MEV opportunities
	mevValue := analyzeTransactionForMEV(tx)
	if mevValue > 0 {
		metrics.Lock()
		metrics.mevDetected++
		metrics.totalFees += mevValue * FEE_PERCENTAGE
		metrics.Unlock()

		log.Printf("💰 MEV Detected: %.6f ETH | Fee: %.6f ETH", mevValue, mevValue*FEE_PERCENTAGE)

		if err := submitFlashbotsBundle(txHex, tx, mevValue); err != nil {
			log.Printf("⚠️  Bundle submission failed: %v", err)
			proxyRequest(w, req, body, nil)
			return
		}
	} else {
		if err := submitFlashbotsBundle(txHex, tx, 0); err != nil {
			log.Printf("⚠️  Private relay failed: %v", err)
			proxyRequest(w, req, body, nil)
			return
		}
	}

	// Respond immediately with the original tx hash for compatibility
	respondSuccess(w, req.ID, tx.Hash().Hex())
}

// handleSendTransaction intercepts signed transactions
func handleSendTransaction(w http.ResponseWriter, req JSONRPCRequest, body []byte, trusted bool, orig *http.Request) {
	var rawParams []interface{}
	if err := json.Unmarshal(req.Params, &rawParams); err == nil && len(rawParams) > 0 {
		if txObj, ok := rawParams[0].(map[string]interface{}); ok {
			if mevValue := analyzeTransactionObjectForMEV(txObj); mevValue > 0 {
				metrics.Lock()
				metrics.mevDetected++
				metrics.totalFees += mevValue * FEE_PERCENTAGE
				metrics.Unlock()
				log.Printf("💰 MEV-like eth_sendTransaction detected: %.6f ETH | fee: %.6f ETH", mevValue, mevValue*FEE_PERCENTAGE)
			}

			if FORCE_PRIVATE_SEND_TX && trusted {
				signedTx, rawTxHex, err := constructSignedTxFromObject(txObj)
				if err == nil {
					log.Printf("🔒 Forced private send: signing and relaying transaction %s", signedTx.Hash().Hex())
					if mevValue := analyzeTransactionForMEV(signedTx); mevValue > 0 {
						metrics.Lock()
						metrics.mevDetected++
						metrics.totalFees += mevValue * FEE_PERCENTAGE
						metrics.Unlock()
					}
					if err := submitFlashbotsBundle(rawTxHex, signedTx, 0); err != nil {
						log.Printf("⚠️ Forced private eth_sendTransaction bundle failed: %v", err)
						proxyRequest(w, req, body, orig)
						return
					}
					respondSuccess(w, req.ID, signedTx.Hash().Hex())
					return
				}
				log.Printf("⚠️ Forced private send enabled but unable to sign tx: %v", err)
			}
		}
	}

	proxyRequest(w, req, body, orig)
}

// analyzeTransactionForMEV detects simple MEV patterns
func analyzeTransactionForMEV(tx *types.Transaction) float64 {
	to := tx.To()
	if to == nil {
		return 0
	}

	if score := detectMEVSignature(tx.Data(), *to); score > 0 {
		return score
	}

	return detectMEVByAddress(*to, tx.Value())
}

func analyzeTransactionObjectForMEV(tx map[string]interface{}) float64 {
	var to common.Address
	if toStr, ok := tx["to"].(string); ok && toStr != "" {
		to = common.HexToAddress(toStr)
	}

	data, _ := tx["data"].(string)
	rawData := []byte{}
	if data != "" {
		if decoded, err := hexutil.Decode(data); err == nil {
			rawData = decoded
		}
	}
	score := detectMEVSignature(rawData, to)
	if score > 0 {
		return score
	}

	value := parseBigInt(tx["value"])
	return detectMEVByAddress(to, value)
}

func detectMEVSignature(data []byte, to common.Address) float64 {
	if len(data) < 4 {
		return 0
	}

	selector := [4]byte{data[0], data[1], data[2], data[3]}
	signatureScores := map[[4]byte]float64{
		{0x41, 0x4b, 0xf3, 0x89}: 0.008, // exactInputSingle
		{0x7f, 0x65, 0xd3, 0x20}: 0.008, // exactOutputSingle
		{0x7f, 0x36, 0xab, 0x5}:  0.007, // swapExactETHForTokens
		{0x38, 0xed, 0x17, 0x39}: 0.007, // swapExactTokensForTokens
		{0x18, 0xcb, 0xaf, 0xe5}: 0.007, // swapExactTokensForETH
		{0xb6, 0x5, 0xed, 0x69}:  0.007, // swapTokensForExactETH
		{0xfb, 0x3b, 0xdb, 0x41}: 0.005, // exactInput
		{0x38, 0xed, 0x17, 0x39}: 0.007, // swapExactTokensForTokens
		{0x9b, 0x8f, 0x92, 0x1b}: 0.006, // swapExactTokensForETHSupportingFeeOnTransferTokens
	}

	if score, ok := signatureScores[selector]; ok {
		return score
	}

	// Detect swaps by target address even when selector is custom
	routerAddrs := map[string]bool{
		UNISWAP_V3.Hex(): true,
		UNISWAP_V2.Hex(): true,
		SUSHISWAP.Hex():  true,
	}

	if routerAddrs[to.Hex()] {
		return 0.004
	}

	return 0
}

func detectMEVByAddress(to common.Address, value *big.Int) float64 {
	if value == nil || value.Sign() == 0 {
		return 0
	}

	if to == UNISWAP_V3 || to == UNISWAP_V2 || to == SUSHISWAP {
		ethValue := new(big.Float).Quo(new(big.Float).SetInt(value), big.NewFloat(1e18))
		estimated, _ := ethValue.Float64()
		if estimated > 0.05 {
			return 0.01
		}
		return 0.005
	}

	return 0
}

func constructSignedTxFromObject(txObj map[string]interface{}) (*types.Transaction, string, error) {
	_, ok := txObj["from"].(string)
	if !ok {
		return nil, "", errors.New("missing from address")
	}

	nonce := parseUint64(txObj["nonce"])
	value := parseBigInt(txObj["value"])
	gasLimit, _ := parseGasLimit(txObj["gas"])
	gasPrice := parseBigInt(txObj["gasPrice"])
	txData := []byte{}
	if dataStr, ok := txObj["data"].(string); ok && dataStr != "" {
		rawData, err := hexutil.Decode(dataStr)
		if err != nil {
			return nil, "", fmt.Errorf("invalid data field: %w", err)
		}
		txData = rawData
	}

	var to *common.Address
	if toStr, ok := txObj["to"].(string); ok && toStr != "" {
		addr := common.HexToAddress(toStr)
		to = &addr
	}

	var tx *types.Transaction
	if to == nil {
		tx = types.NewTransaction(nonce, common.Address{}, value, gasLimit, gasPrice, txData)
	} else {
		tx = types.NewTransaction(nonce, *to, value, gasLimit, gasPrice, txData)
	}

	signer := types.NewEIP155Signer(big.NewInt(MAINNET_ID))
	signedTx, err := types.SignTx(tx, signer, backrunKey)
	if err != nil {
		return nil, "", err
	}

	txBytes, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, "", err
	}

	return signedTx, hexutil.Encode(txBytes), nil
}

func parseGasLimit(val interface{}) (uint64, error) {
	if val == nil {
		return 21000, nil
	}
	if s, ok := val.(string); ok {
		if strings.HasPrefix(s, "0x") {
			return parseUint64(s), nil
		}
		var g uint64
		fmt.Sscanf(s, "%d", &g)
		return g, nil
	}
	if f, ok := val.(float64); ok {
		return uint64(f), nil
	}
	return 21000, nil
}

// calculateDynamicFee adjusts fee based on MEV opportunity magnitude
func calculateDynamicFee(mevValue float64) float64 {
	if mevValue < 0.001 {
		return 0.05 // 5% for small MEV
	}
	if mevValue < 0.01 {
		return 0.08 // 8% for medium MEV
	}
	if mevValue < 0.1 {
		return 0.12 // 12% for large MEV
	}
	return 0.15 // 15% for massive MEV opportunities
}

// isSandwichAttack detects if a transaction is likely being sandwiched
func isSandwichAttack(tx *types.Transaction) bool {
	if !ENABLE_SANDWICH_PROTECTION {
		return false
	}

	// Check for common sandwich patterns
	to := tx.To()
	if to == nil {
		return false
	}

	// If targeting swap routers with suspicious data patterns
	isSwapRouter := *to == UNISWAP_V3 || *to == UNISWAP_V2 || *to == SUSHISWAP

	if !isSwapRouter {
		return false
	}

	// Heuristic: high gas price relative to network average suggests frontrunning
	gasPrice := tx.GasPrice()
	if gasPrice == nil {
		return false
	}

	upstreamGasPrice := parseBigInt(callUpstreamRPC("eth_gasPrice", []interface{}{}))
	if upstreamGasPrice == nil || upstreamGasPrice.Sign() == 0 {
		return false
	}

	ratio := new(big.Float).Quo(
		new(big.Float).SetInt(gasPrice),
		new(big.Float).SetInt(upstreamGasPrice),
	)
	ratioVal, _ := ratio.Float64()

	// If tx pays >3x normal gas price, likely sandwich attempt
	return ratioVal > 3.0
}

func submitFlashbotsBundle(rawTxHex string, originalTx *types.Transaction, mevValue float64) error {
	// Sandwich attack protection
	if originalTx != nil && isSandwichAttack(originalTx) {
		metrics.Lock()
		metrics.sandwichAttacksBlocked++
		metrics.Unlock()
		log.Printf("🛡️  Sandwich attack detected and blocked | gas price ratio suspicious")
		// Still relay through private channels but don't extract fee
		mevValue = 0
	}

	bundleTxs := []string{rawTxHex}
	var feeAmount *big.Int
	var gasPrice *big.Int

	if originalTx != nil && mevValue > 0 {
		nonce := callUpstreamRPC("eth_getTransactionCount", []interface{}{backrunAddr.Hex(), "pending"})
		gpRaw := callUpstreamRPC("eth_gasPrice", []interface{}{})

		gasPrice = parseBigInt(gpRaw)

		// Use dynamic fee calculation instead of fixed percentage
		dynamicFee := calculateDynamicFee(mevValue)
		feeWei := big.NewFloat(mevValue * dynamicFee * 1e18)
		feeAmount = new(big.Int)
		feeWei.Int(feeAmount)

		// Ensure minimum fee
		if feeAmount.Sign() == 0 {
			feeAmount = new(big.Int).Mul(big.NewInt(1), big.NewInt(1e15)) // 0.001 ETH minimum
		}

		backrunTx := types.NewTransaction(
			parseUint64(nonce),
			common.HexToAddress(FEE_RECIPIENT),
			feeAmount,
			21000,
			gasPrice,
			nil,
		)

		signer := types.NewEIP155Signer(big.NewInt(MAINNET_ID))
		signedTx, err := types.SignTx(backrunTx, signer, backrunKey)
		if err != nil {
			metrics.Lock()
			metrics.bundlesFailed++
			metrics.Unlock()
			return fmt.Errorf("failed to sign backrun tx: %w", err)
		}

		backrunTxBytes, _ := signedTx.MarshalBinary()
		bundleTxs = append(bundleTxs, hexutil.Encode(backrunTxBytes))
	}

	blockNumber := callUpstreamRPC("eth_blockNumber", []interface{}{})
	targetBlock := fmt.Sprintf("%s", blockNumber)
	if targetBlock == "" {
		targetBlock = "latest"
	}

	bundle := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      time.Now().Unix(),
		"method":  "eth_sendBundle",
		"params": []interface{}{
			map[string]interface{}{
				"txs":               bundleTxs,
				"blockNumber":       targetBlock,
				"revertingTxHashes": []string{},
			},
		},
	}

	payload, _ := json.Marshal(bundle)

	// Try all builder relays in parallel
	successChan := make(chan bool, len(BUILDER_RELAYS))
	for _, relayURL := range BUILDER_RELAYS {
		go func(relay string) {
			if err := sendBundleToRelay(relay, payload, backrunKey, backrunAddr); err == nil {
				successChan <- true
			} else {
				log.Printf("⚠️  Builder %s failed: %v", relay, err)
				successChan <- false
			}
		}(relayURL)
	}

	// Collect results
	successCount := 0
	for i := 0; i < len(BUILDER_RELAYS); i++ {
		if <-successChan {
			successCount++
		}
	}

	metrics.Lock()
	metrics.flashbotsBundlesSent++
	if successCount > 0 {
		metrics.flashbotsBundlesOK++
		metrics.multiBuilderSent++
	} else {
		metrics.bundlesFailed++
	}
	metrics.Unlock()

	if successCount == 0 {
		// Fallback to mempool
		if rawTxHex != "" {
			up := callUpstreamRPC("eth_sendRawTransaction", []interface{}{rawTxHex})
			if up != nil {
				metrics.Lock()
				metrics.flashbotsFallbacks++
				metrics.Unlock()
				log.Printf("🔁 All builders failed; mempool fallback accepted: %v", up)
				return nil
			}
		}
		return fmt.Errorf("all builder relays failed")
	}

	feeStr := "0"
	if feeAmount != nil {
		feeEth := new(big.Float).Quo(new(big.Float).SetInt(feeAmount), big.NewFloat(1e18))
		feeStr = feeEth.String()
	}

	log.Printf("✅ Bundle to %d builders | txs=%d | mev=%.6f ETH | fee=%s ETH | shield=%s",
		successCount, len(bundleTxs), mevValue, feeStr, map[bool]string{true: "enabled", false: "disabled"}[ENABLE_SANDWICH_PROTECTION])
	return nil
}

// sendBundleToRelay sends a bundle to a single relay
func sendBundleToRelay(relayURL string, payload []byte, key *ecdsa.PrivateKey, addr common.Address) error {
	req, _ := http.NewRequest("POST", relayURL, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	// Sign with builder key
	sigPayload := fmt.Sprintf("%d:%s", time.Now().Unix(), hexutil.Encode(payload))
	sig, _ := crypto.Sign(crypto.Keccak256([]byte(sigPayload)), key)
	req.Header.Set("X-Flashbots-Signature", fmt.Sprintf("%s:%s", addr.Hex(), hexutil.Encode(sig)))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("relay returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// proxyRequest forwards request to upstream RPC
func proxyRequest(w http.ResponseWriter, rpcReq JSONRPCRequest, body []byte, orig *http.Request) {
	ctx := context.Background()
	if orig != nil {
		ctx = orig.Context()
	}
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", UPSTREAM_RPC, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	if orig != nil {
		for _, header := range []string{"Authorization", "User-Agent", CLIENT_HEADER_NAME, "X-Request-ID"} {
			if value := orig.Header.Get(header); value != "" {
				httpReq.Header.Set(header, value)
			}
		}
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		respondError(w, rpcReq.ID, "Upstream error", -32603)
		return
	}
	defer resp.Body.Close()

	// Read and forward response
	respBody, _ := io.ReadAll(resp.Body)
	w.Write(respBody)
}

// Utility response functions
func respondError(w http.ResponseWriter, id interface{}, message string, code int) {
	response := JSONRPCResponse{
		Jsonrpc: "2.0",
		Error: map[string]interface{}{
			"code":    code,
			"message": message,
		},
		ID: id,
	}
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}

func respondSuccess(w http.ResponseWriter, id interface{}, result interface{}) {
	response := JSONRPCResponse{
		Jsonrpc: "2.0",
		Result:  result,
		ID:      id,
	}
	json.NewEncoder(w).Encode(response)
}

func isTrustedClient(r *http.Request) bool {
	clientID := strings.TrimSpace(r.Header.Get(CLIENT_HEADER_NAME))
	if clientID == "" {
		return false
	}
	return trustedClients[clientID]
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	metrics.Lock()
	defer metrics.Unlock()

	successRate := 0.0
	if metrics.flashbotsBundlesSent > 0 {
		successRate = float64(metrics.flashbotsBundlesOK) / float64(metrics.flashbotsBundlesSent) * 100
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalFeesCaptured":        fmt.Sprintf("%.6f ETH", metrics.totalFees),
		"transactionsProcessed":    metrics.txProcessed,
		"trustedRequests":          metrics.trustedRequests,
		"mevOpportunitiesDetected": metrics.mevDetected,
		"backrunsExecuted":         metrics.backrunsExec,
		"flashbotsBundlesSent":     metrics.flashbotsBundlesSent,
		"flashbotsBundlesOK":       metrics.flashbotsBundlesOK,
		"bundleSuccessRate":        fmt.Sprintf("%.2f%%", successRate),
		"flashbotsFallbacks":       metrics.flashbotsFallbacks,
		"bundlesFailed":            metrics.bundlesFailed,
		"sandwichAttacksBlocked":   metrics.sandwichAttacksBlocked,
		"multiBuilderSubmissions":  metrics.multiBuilderSent,
		"backrunSignerAddress":     backrunAddr.Hex(),
		"feeRecipient":             FEE_RECIPIENT,
		"clientCounts":             metrics.clientCounts,
		"sandwichProtection":       ENABLE_SANDWICH_PROTECTION,
		"timestamp":                time.Now().Unix(),
	})
}

// callUpstreamRPC makes a JSON-RPC call to upstream
func callUpstreamRPC(method string, params []interface{}) interface{} {
	payload := JSONRPCRequest{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  nil,
		ID:      1,
	}

	paramBytes, _ := json.Marshal(params)
	payload.Params = paramBytes

	reqBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", UPSTREAM_RPC, bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var result JSONRPCResponse
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &result)
	return result.Result
}

// parseUint64 converts hex string to uint64
func parseUint64(val interface{}) uint64 {
	if val == nil {
		return 0
	}
	str := fmt.Sprintf("%v", val)
	if len(str) > 2 && str[:2] == "0x" {
		str = str[2:]
	}
	var result uint64
	fmt.Sscanf(str, "%x", &result)
	return result
}

// parseBigInt converts hex string to *big.Int
func parseBigInt(val interface{}) *big.Int {
	if val == nil {
		return big.NewInt(0)
	}
	str := fmt.Sprintf("%v", val)
	if len(str) > 2 && str[:2] == "0x" {
		str = str[2:]
	}
	result := new(big.Int)
	result.SetString(str, 16)
	return result
}
