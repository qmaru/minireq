package minireq

import (
	"bytes"
	"encoding/json"
)

// JSONCodec defines the interface for JSON encoding/decoding operations
type JSONCodec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
	UnmarshalNumber(data []byte, v any) error
	Valid(data []byte) bool
}

// defaultJSONCodec wraps encoding/json as the default implementation
type defaultJSONCodec struct{}

func (d defaultJSONCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (d defaultJSONCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (d defaultJSONCodec) UnmarshalNumber(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(v)
}

func (d defaultJSONCodec) Valid(data []byte) bool {
	return json.Valid(data)
}

// DefaultJSONCodec is the package-level default codec
var DefaultJSONCodec JSONCodec = defaultJSONCodec{}
