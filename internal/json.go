package internal

import (
	"encoding/json"
	"strings"
	"time"
)

var _ json.Marshaler = &JSONTime{}
var _ json.Unmarshaler = &JSONTime{}

type JSONTime time.Time

const timeFormat = "2006-01-02"

func (t *JSONTime) UnmarshalJSON(bytes []byte) error {
	field := strings.ReplaceAll(string(bytes), `"`, "")
	parsed, err := time.Parse(timeFormat, field)
	if err != nil {
		return err
	}
	*t = JSONTime(parsed)
	return nil
}

func (t JSONTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).Format(timeFormat))
}
