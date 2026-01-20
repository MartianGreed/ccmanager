package usage

import "strings"

// ModelPricing represents pricing for a specific model (per 1M tokens)
type ModelPricing struct {
	Input      float64
	Output     float64
	CacheWrite float64
	CacheRead  float64
}

// Pricing table per 1M tokens
var modelPricing = map[string]ModelPricing{
	"opus": {
		Input:      15.0,
		Output:     75.0,
		CacheWrite: 18.75,
		CacheRead:  1.50,
	},
	"sonnet": {
		Input:      3.0,
		Output:     15.0,
		CacheWrite: 3.75,
		CacheRead:  0.30,
	},
	"haiku": {
		Input:      0.80,
		Output:     4.0,
		CacheWrite: 1.0,
		CacheRead:  0.08,
	},
}

// NormalizeModelName converts various model ID formats to our internal name
func NormalizeModelName(model string) string {
	model = strings.ToLower(model)

	if strings.Contains(model, "opus") {
		return "opus"
	}
	if strings.Contains(model, "sonnet") {
		return "sonnet"
	}
	if strings.Contains(model, "haiku") {
		return "haiku"
	}

	// Default to sonnet if unknown
	return "sonnet"
}

// CalculateCost calculates the estimated cost for token usage
func CalculateCost(usage TokenUsage, model string) float64 {
	normalizedModel := NormalizeModelName(model)
	pricing, ok := modelPricing[normalizedModel]
	if !ok {
		pricing = modelPricing["sonnet"]
	}

	// Convert tokens to millions
	inputM := float64(usage.InputTokens) / 1_000_000
	outputM := float64(usage.OutputTokens) / 1_000_000
	cacheWriteM := float64(usage.CacheCreationInputTokens) / 1_000_000
	cacheReadM := float64(usage.CacheReadInputTokens) / 1_000_000

	cost := inputM*pricing.Input +
		outputM*pricing.Output +
		cacheWriteM*pricing.CacheWrite +
		cacheReadM*pricing.CacheRead

	return cost
}

// GetPricing returns the pricing for a given model
func GetPricing(model string) ModelPricing {
	normalizedModel := NormalizeModelName(model)
	pricing, ok := modelPricing[normalizedModel]
	if !ok {
		return modelPricing["sonnet"]
	}
	return pricing
}
