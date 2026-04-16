package execution

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// SignBinanceParams generates an HMAC-SHA256 hex signature for Binance WebSocket API params.
//
// Algorithm:
//  1. Collect all parameter keys except "signature".
//  2. Sort keys alphabetically.
//  3. Build a query string: key1=value1&key2=value2&...
//  4. HMAC-SHA256 the query string with apiSecret as the key.
//  5. Return the hex-encoded digest.
func SignBinanceParams(params map[string]string, apiSecret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "signature" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	queryString := strings.Join(parts, "&")

	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(queryString))
	return hex.EncodeToString(mac.Sum(nil))
}
