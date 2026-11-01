package jsonl

import "encoding/json"

// Legacy NDJSON is a metadata contract, not an arbitrary content archive.
func metadataPayload(payload json.RawMessage) json.RawMessage {
	var input map[string]json.RawMessage
	if json.Unmarshal(payload, &input) != nil {
		return json.RawMessage("{}")
	}
	allowed := make(map[string]json.RawMessage)
	for _, key := range []string{"provider", "model", "model_id", "agent_name", "capture_quality", "task_id", "turn_id", "interaction_upstream_id", "prompt_hash", "interaction_hash", "interaction_redacted", "prompt_capture_mode", "session_id", "message_id", "parent_message_id", "part_id", "finish", "tool_name", "copilot_window_session_id", "prompt_source"} {
		var value string
		if raw, ok := input[key]; ok && json.Unmarshal(raw, &value) == nil {
			allowed[key] = raw
		}
	}
	for _, key := range []string{"input_tokens", "output_tokens", "reasoning_tokens", "cached_input_tokens", "cache_write_tokens", "total_tokens", "tokens", "estimated_cost_usd_micros", "estimated_cost_eur_micros", "created_at", "completed_at"} {
		var value int64
		if raw, ok := input[key]; ok && json.Unmarshal(raw, &value) == nil && value >= 0 {
			allowed[key] = raw
		}
	}
	for _, key := range []string{"prompt_available", "codex_response_completed"} {
		var value bool
		if raw, ok := input[key]; ok && json.Unmarshal(raw, &value) == nil {
			allowed[key] = raw
		}
	}
	var observations []metricObservation
	if json.Unmarshal(input["metric_observations"], &observations) == nil {
		filtered := make([]metricObservation, 0, len(observations))
		for _, observation := range observations {
			switch observation.Name {
			case "input_tokens", "output_tokens", "reasoning_tokens", "cached_input_tokens", "cache_write_tokens", "total_tokens":
				if observation.Value != nil && *observation.Value >= 0 {
					filtered = append(filtered, observation)
				}
			}
		}
		if len(filtered) > 0 {
			allowed["metric_observations"], _ = json.Marshal(filtered)
		}
	}
	result, _ := json.Marshal(allowed)
	return result
}

func metadataEvidence(payload json.RawMessage) json.RawMessage {
	var input map[string]json.RawMessage
	_ = json.Unmarshal(payload, &input)
	allowed := make(map[string]string)
	for _, key := range []string{"source", "method", "confidence"} {
		var value string
		if raw, ok := input[key]; ok && json.Unmarshal(raw, &value) == nil {
			allowed[key] = value
		}
	}
	result, _ := json.Marshal(allowed)
	return result
}
