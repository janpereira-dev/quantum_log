package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
)

func addUsageCount(left, right int64) (int64, error) {
	if left < 0 || right < 0 {
		return 0, errors.New("usage counts must not be negative; existing evidence requires explicit repair")
	}
	if left > math.MaxInt64-right {
		return 0, errors.New("usage count overflow: aggregate exceeds the supported integer range")
	}
	return left + right, nil
}

func modelCallTotal(input ModelCallInput) (int64, error) {
	var components int64
	for _, value := range []int64{input.InputTokens, input.OutputTokens, input.ReasoningTokens, input.CachedInputTokens, input.CacheWriteTokens} {
		var err error
		components, err = addUsageCount(components, value)
		if err != nil {
			return 0, err
		}
	}
	total := components
	for _, metric := range input.Metrics {
		if metric.Name == "total_tokens" && metric.Value != nil {
			total = *metric.Value
		}
	}
	if total < components {
		return 0, errors.New("reported total tokens must cover the known disjoint token components")
	}
	for _, metric := range input.Metrics {
		if metric.Value != nil && (*metric.Value < 0 || *metric.Value > total) {
			return 0, errors.New("metric observation must be non-negative and cannot exceed total tokens")
		}
	}
	return total, nil
}

func (s *Store) nextUsageTotal(ctx context.Context, tx *sql.Tx, total int64) (int64, error) {
	if s.recordedTokens == nil {
		var recorded int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_tokens),0) FROM model_calls`).Scan(&recorded); err != nil {
			return 0, fmt.Errorf("validate existing aggregate token capacity: %w", err)
		}
		if recorded < 0 {
			return 0, errors.New("existing aggregate tokens are negative; explicit repair is required")
		}
		s.recordedTokens = &recorded
	}
	return addUsageCount(*s.recordedTokens, total)
}

func addUsageCounts(targets [7]*int64, values [7]int64) error {
	for index, value := range values {
		next, err := addUsageCount(*targets[index], value)
		if err != nil {
			return err
		}
		*targets[index] = next
	}
	return nil
}

func validateAllocationRange(offset, basisPoints int64) error {
	if offset < 0 || basisPoints < 0 || basisPoints > 10000 || offset > 10000-basisPoints {
		return errors.New("stored allocation exceeds 10000 basis points; explicit repair is required")
	}
	return nil
}
