package models

import "encoding/json"

type Event struct {
	BlockId uint64          `json:"block_id"`
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data"`
}
