package types

import (
    "database/sql/driver"
    "encoding/json"
    "fmt"
)

// StringSlice stores a JSON array in a TEXT column.
// Implements sql.Scanner and driver.Valuer so database/sql can persist it.
type StringSlice []string

// Scan implements the sql.Scanner interface.
func (s *StringSlice) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*s = nil
		return nil
	case []byte:
		if len(v) == 0 {
			*s = nil
			return nil
		}
		return json.Unmarshal(v, s)
	case string:
		if v == "" {
			*s = nil
			return nil
		}
		return json.Unmarshal([]byte(v), s)
	default:
		return fmt.Errorf("types.StringSlice: unsupported Scan type %T", src)
	}
}

// Value implements the driver.Valuer interface.
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	b, err := json.Marshal([]string(s))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}
