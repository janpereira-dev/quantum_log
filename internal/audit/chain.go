// Package audit provides deterministic hash-chain verification for local ledgers.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type Record struct {
	SessionID    string
	Payload      string
	PreviousHash string
	Hash         string
}

func NewRecord(sessionID, payload, previousHash string) Record {
	record := Record{SessionID: sessionID, Payload: payload, PreviousHash: previousHash}
	record.Hash = Hash(record.SessionID, record.Payload, record.PreviousHash)
	return record
}

func Hash(sessionID, payload, previousHash string) string {
	sum := sha256.Sum256([]byte(sessionID + "\n" + payload + "\n" + previousHash))
	return hex.EncodeToString(sum[:])
}

func Verify(records []Record) error {
	previousBySession := make(map[string]string)
	for index, record := range records {
		if record.PreviousHash != previousBySession[record.SessionID] {
			return fmt.Errorf("record %d has an invalid previous hash", index)
		}
		if record.Hash != Hash(record.SessionID, record.Payload, record.PreviousHash) {
			return fmt.Errorf("record %d hash does not match payload", index)
		}
		previousBySession[record.SessionID] = record.Hash
	}
	return nil
}
