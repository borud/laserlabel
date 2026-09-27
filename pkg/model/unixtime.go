package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// UnixTime is a time.Time that is stored as a Unix epoch integer.
type UnixTime time.Time

// Time returns the underlying time.Time.
func (t UnixTime) Time() time.Time { return time.Time(t) }

// Value implements driver.Valuer.
func (t UnixTime) Value() (driver.Value, error) {
	return time.Time(t).Unix(), nil
}

// Scan implements sql.Scanner.
func (t *UnixTime) Scan(src any) error {
	switch v := src.(type) {
	case int64:
		*t = UnixTime(time.Unix(v, 0))
		return nil
	case nil:
		*t = UnixTime(time.Time{})
		return nil
	}
	return fmt.Errorf("cannot scan %T into UnixTime", src)
}

// MarshalJSON encodes the time in RFC 3339 format.
func (t UnixTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t))
}

// UnmarshalJSON decodes an RFC 3339 time.
func (t *UnixTime) UnmarshalJSON(b []byte) error {
	var tt time.Time
	err := json.Unmarshal(b, &tt)
	if err != nil {
		return err
	}
	*t = UnixTime(tt)
	return nil
}
