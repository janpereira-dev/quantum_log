// Package pricing validates versioned token pricing rules and calculates integer micros.
package pricing

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

type Rule struct {
	SchemaVersion int        `json:"schemaVersion"`
	Provider      string     `json:"provider"`
	ModelPattern  string     `json:"modelPattern"`
	ValidFrom     time.Time  `json:"validFrom"`
	ValidUntil    *time.Time `json:"validUntil"`
	BillingMode   string     `json:"billingMode"`
	Currency      string     `json:"currency"`
	UnitTokens    int64      `json:"unitTokens"`
	Prices        Prices     `json:"prices"`
	Version       string     `json:"version"`
}

type Prices struct {
	InputMicros       int64 `json:"inputMicros"`
	CachedInputMicros int64 `json:"cachedInputMicros"`
	CacheWriteMicros  int64 `json:"cacheWriteMicros"`
	OutputMicros      int64 `json:"outputMicros"`
	ReasoningMicros   int64 `json:"reasoningMicros"`
}

type Usage struct {
	InputTokens       int64
	CachedInputTokens int64
	CacheWriteTokens  int64
	OutputTokens      int64
	ReasoningTokens   int64
}

func Load(reader io.Reader) (Rule, error) {
	var rule Rule
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rule); err != nil {
		return Rule{}, fmt.Errorf("decode pricing rule: %w", err)
	}
	if rule.SchemaVersion != 1 || rule.Provider == "" || rule.ModelPattern == "" || rule.BillingMode != "token" || rule.Currency != "USD" || rule.UnitTokens <= 0 || rule.Version == "" || rule.ValidFrom.IsZero() {
		return Rule{}, errors.New("invalid pricing rule")
	}
	return rule, nil
}

func Calculate(rule Rule, usage Usage, occurredAt time.Time) (int64, error) {
	if occurredAt.Before(rule.ValidFrom) || rule.ValidUntil != nil && !occurredAt.Before(*rule.ValidUntil) {
		return 0, errors.New("pricing rule is not valid at occurrence time")
	}
	if rule.BillingMode != "token" || rule.UnitTokens <= 0 {
		return 0, errors.New("unsupported pricing rule")
	}
	cost := usage.InputTokens*rule.Prices.InputMicros + usage.CachedInputTokens*rule.Prices.CachedInputMicros + usage.CacheWriteTokens*rule.Prices.CacheWriteMicros + usage.OutputTokens*rule.Prices.OutputMicros + usage.ReasoningTokens*rule.Prices.ReasoningMicros
	return cost / rule.UnitTokens, nil
}
