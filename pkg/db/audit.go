package db

import (
	"time"
)

type AuditEntry struct {
	User        string
	Action      string
	Resource    string
	Details     string
	LLMRequest  string
	LLMResponse string
}

func RecordAudit(entry AuditEntry) error {
	if DB == nil {
		return nil
	}

	query := `INSERT INTO audit_logs (timestamp, user, action, resource, details, llm_request, llm_response) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := DB.Exec(query, time.Now(), entry.User, entry.Action, entry.Resource, entry.Details, entry.LLMRequest, entry.LLMResponse)
	return err
}

func GetAuditLogs() ([]map[string]interface{}, error) {
	if DB == nil {
		return nil, nil
	}

	rows, err := DB.Query("SELECT id, timestamp, user, action, resource, details FROM audit_logs ORDER BY timestamp DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var id int
		var ts time.Time
		var user, action, res, details string
		if err := rows.Scan(&id, &ts, &user, &action, &res, &details); err != nil {
			return nil, err
		}
		logs = append(logs, map[string]interface{}{
			"id":        id,
			"timestamp": ts,
			"user":      user,
			"action":    action,
			"resource":  res,
			"details":   details,
		})
	}
	return logs, nil
}
