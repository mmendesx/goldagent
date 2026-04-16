package execution

import (
	"testing"
)

// TestSignBinanceParams_KnownVector verifies the signature against a reference computed offline.
//
// Reference computation (Python):
//
//	import hmac, hashlib
//	msg = b"apiKey=testKey&quantity=0.001&side=BUY&symbol=BTCUSDT&timestamp=1234567890000&type=MARKET"
//	print(hmac.new(b"testSecret", msg, hashlib.sha256).hexdigest())
//	# => abc6eb8c6a32b651f6e9bb4e370aaba5e9227711e19e49d20a1a875af449589e
func TestSignBinanceParams_KnownVector(t *testing.T) {
	params := map[string]string{
		"symbol":    "BTCUSDT",
		"side":      "BUY",
		"type":      "MARKET",
		"quantity":  "0.001",
		"apiKey":    "testKey",
		"timestamp": "1234567890000",
	}

	const (
		apiSecret         = "testSecret"
		expectedSignature = "abc6eb8c6a32b651f6e9bb4e370aaba5e9227711e19e49d20a1a875af449589e"
	)

	got := SignBinanceParams(params, apiSecret)

	if got != expectedSignature {
		t.Errorf("SignBinanceParams mismatch:\n  got:  %q\n  want: %q", got, expectedSignature)
	}
}

// TestSignBinanceParams_DeterministicAndCorrect checks that:
// 1. The same inputs always produce the same signature (deterministic).
// 2. Adding "signature" to params does not affect the output (signature key is excluded).
func TestSignBinanceParams_DeterministicAndCorrect(t *testing.T) {
	params := map[string]string{
		"symbol":    "BTCUSDT",
		"side":      "BUY",
		"type":      "MARKET",
		"quantity":  "0.001",
		"apiKey":    "testKey",
		"timestamp": "1234567890000",
	}
	const secret = "testSecret"

	sig1 := SignBinanceParams(params, secret)
	sig2 := SignBinanceParams(params, secret)

	if sig1 != sig2 {
		t.Errorf("SignBinanceParams is not deterministic: got %q then %q", sig1, sig2)
	}

	if len(sig1) != 64 {
		t.Errorf("expected 64-char hex signature, got %d: %q", len(sig1), sig1)
	}

	// Adding "signature" key to params must not change the output.
	paramsWithSig := make(map[string]string, len(params)+1)
	for k, v := range params {
		paramsWithSig[k] = v
	}
	paramsWithSig["signature"] = "someprevioussig"
	sigWithExtra := SignBinanceParams(paramsWithSig, secret)

	if sigWithExtra != sig1 {
		t.Errorf("signature key in params should be excluded from signing: got %q, want %q", sigWithExtra, sig1)
	}
}

// TestSignBinanceParams_ParameterOrdering verifies that different map insertion orders
// produce the same signature (the implementation sorts before signing).
func TestSignBinanceParams_ParameterOrdering(t *testing.T) {
	const secret = "mySecret"

	paramsA := map[string]string{
		"z": "last",
		"a": "first",
		"m": "middle",
	}
	paramsB := map[string]string{
		"m": "middle",
		"z": "last",
		"a": "first",
	}

	sigA := SignBinanceParams(paramsA, secret)
	sigB := SignBinanceParams(paramsB, secret)

	if sigA != sigB {
		t.Errorf("same params in different orders produced different signatures: %q vs %q", sigA, sigB)
	}
}

// TestSignBinanceParams_DifferentSecrets verifies that different API secrets produce
// different signatures for the same params (HMAC key sensitivity).
func TestSignBinanceParams_DifferentSecrets(t *testing.T) {
	params := map[string]string{
		"symbol":    "BTCUSDT",
		"timestamp": "1000000000000",
	}

	sig1 := SignBinanceParams(params, "secretA")
	sig2 := SignBinanceParams(params, "secretB")

	if sig1 == sig2 {
		t.Error("different API secrets produced the same signature — HMAC key is not being applied")
	}
}

// TestSignBinanceParams_EmptyParams verifies the function handles an empty param map
// without panicking and returns a valid 64-char hex signature.
func TestSignBinanceParams_EmptyParams(t *testing.T) {
	sig := SignBinanceParams(map[string]string{}, "secret")
	if len(sig) != 64 {
		t.Errorf("empty params: expected 64-char hex signature, got %d: %q", len(sig), sig)
	}
}
