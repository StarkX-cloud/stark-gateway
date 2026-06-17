package main

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestAnalyzeTransactionObjectForMEV(t *testing.T) {
	data := "0x414bf389" + strings.Repeat("00", 32)
	txObj := map[string]interface{}{"data": data}
	v := analyzeTransactionObjectForMEV(txObj)
	if v <= 0 {
		t.Fatalf("expected MEV detected >0, got %v", v)
	}
}

func TestConstructSignedTxFromObject(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	backrunKey = key
	backrunAddr = crypto.PubkeyToAddress(key.PublicKey)

	txObj := map[string]interface{}{
		"from":     backrunAddr.Hex(),
		"to":       "0x0000000000000000000000000000000000000001",
		"nonce":    "0x0",
		"value":    "0x0",
		"gas":      "0x5208",
		"gasPrice": "0x3b9aca00",
		"data":     "0x",
	}

	signedTx, raw, err := constructSignedTxFromObject(txObj)
	if err != nil {
		t.Fatalf("constructSignedTxFromObject error: %v", err)
	}
	if signedTx == nil || raw == "" {
		t.Fatalf("expected signed tx and raw hex")
	}
}
