package pricing

import (
	"strings"
	"testing"
	"time"
)

func TestLoadAndCalculateTokenCost(t *testing.T) {
	rule, err := Load(strings.NewReader(`{"schemaVersion":1,"provider":"example","modelPattern":"example-pro","validFrom":"2026-07-01T00:00:00Z","billingMode":"token","currency":"USD","unitTokens":1000000,"prices":{"inputMicros":3000000,"cachedInputMicros":750000,"cacheWriteMicros":0,"outputMicros":15000000,"reasoningMicros":0},"version":"2026.07.1"}`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got, err := Calculate(rule, Usage{InputTokens: 1_000_000, CachedInputTokens: 1_000_000, OutputTokens: 1_000_000}, time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if got != 18_750_000 {
		t.Fatalf("Calculate() = %d, want 18750000", got)
	}
}

func TestCalculateRejectsRuleOutsideValidity(t *testing.T) {
	rule := Rule{Provider: "example", ModelPattern: "model", ValidFrom: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), BillingMode: "token", UnitTokens: 1, Prices: Prices{InputMicros: 1}}
	if _, err := Calculate(rule, Usage{InputTokens: 1}, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("Calculate() accepted a rule before validFrom")
	}
}
