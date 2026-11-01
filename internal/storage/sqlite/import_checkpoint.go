package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// ModelCallImportCheckpoint reads recorded evidence without changing prior intervals.
func (s *Store) ModelCallImportCheckpoint(ctx context.Context, source, sessionID, identity string) (time.Time, json.RawMessage, bool, error) {
	canonical, err := CanonicalIngestionIdentity(RawEventInput{Source: source, SessionID: sessionID, IngestionIdentity: identity}, nil)
	if err != nil {
		return time.Time{}, nil, false, err
	}
	var duplicate bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM raw_event_dedup d JOIN raw_events r ON r.id=d.raw_event_id JOIN model_calls c ON c.raw_event_id=r.id WHERE d.ingestion_identity=? AND r.source=? AND r.session_id=?)`, canonical, source, sessionID).Scan(&duplicate); err != nil {
		return time.Time{}, nil, false, err
	}
	if duplicate {
		return time.Time{}, nil, true, nil
	}
	var at, payload string
	err = s.db.QueryRowContext(ctx, `SELECT r.occurred_at,r.payload_json_sanitized FROM raw_events r WHERE r.source=? AND r.session_id=? AND EXISTS(SELECT 1 FROM model_calls c WHERE c.raw_event_id=r.id) ORDER BY r.occurred_at DESC LIMIT 1`, source, sessionID).Scan(&at, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil, false, nil
	}
	return parseTimestamp(at), json.RawMessage(payload), false, err
}
