package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONMap is a string-to-string map stored as a JSON object in a text
// column. It holds free key/value data, such as Cachet component meta.
type JSONMap map[string]string

// Value implements driver.Valuer. An empty map is stored as NULL.
func (m JSONMap) Value() (driver.Value, error) {
	if len(m) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(map[string]string(m))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (m *JSONMap) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*m = nil
		return nil
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return fmt.Errorf("JSONMap: cannot scan %T", src)
	}
	if len(raw) == 0 {
		*m = nil
		return nil
	}
	out := map[string]string{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("JSONMap: %w", err)
	}
	*m = out
	return nil
}
