package bi

import (
	"math/big"
	"regexp"
)

type RawTokens struct {
	Encoding   string  `json:"encoding"`
	Input      *string `json:"input"`
	Output     *string `json:"output"`
	CacheRead  *string `json:"cache_read"`
	CacheWrite *string `json:"cache_write"`
}

type NormalizedTokens struct {
	Input      *string
	Output     *string
	CacheRead  *string
	CacheWrite *string
}

var tokenPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

func NormalizeTokens(raw RawTokens) (NormalizedTokens, error) {
	values := []*string{raw.Input, raw.Output, raw.CacheRead, raw.CacheWrite}
	if raw.Encoding == "unavailable" {
		for _, value := range values {
			if value != nil {
				return NormalizedTokens{}, invalid("tokens", "Unavailable token counts must all be null")
			}
		}
		return NormalizedTokens{}, nil
	}
	if raw.Encoding != "exclusive_buckets" && raw.Encoding != "inclusive_input" {
		return NormalizedTokens{}, invalid("tokens.encoding", "Unknown encoding")
	}
	parsed := make([]*big.Int, 4)
	for i, value := range values {
		if value == nil || len(*value) > 131072 || !tokenPattern.MatchString(*value) {
			return NormalizedTokens{}, invalid("tokens", "Expected non-negative integer strings")
		}
		parsed[i] = new(big.Int)
		parsed[i].SetString(*value, 10)
	}
	cache := new(big.Int).Add(parsed[2], parsed[3])
	if raw.Encoding == "exclusive_buckets" {
		parsed[0].Add(parsed[0], cache)
	} else if parsed[0].Cmp(cache) < 0 {
		return NormalizedTokens{}, invalid("tokens", "Cache buckets exceed inclusive input")
	}
	input, output, read, write := parsed[0].String(), parsed[1].String(), parsed[2].String(), parsed[3].String()
	if len(input) > 131072 {
		return NormalizedTokens{}, invalid("tokens", "Token count exceeds numeric storage capacity")
	}
	return NormalizedTokens{Input: &input, Output: &output, CacheRead: &read, CacheWrite: &write}, nil
}
