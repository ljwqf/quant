package strategy

import "strings"

func normalizeMarketSymbol(symbol string) string {
	normalized := strings.ToUpper(strings.TrimSpace(symbol))
	replacer := strings.NewReplacer("-", "", "_", "", "/", "")
	return replacer.Replace(normalized)
}
