package minireq

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// DefaultVer Library version
const DefaultVer = "2.0.0"

// DefaultUA Default User-Agent
const DefaultUA = "MiniRequest/" + DefaultVer

type MultipartMode int64

const (
	Buffered MultipartMode = iota
	Streaming
)

// Auth Set HTTP Basic Auth
type Auth struct {
	Username string
	Password string
}

func (a *Auth) IsEmpty() bool {
	return a == nil || (a.Username == "" && a.Password == "")
}

// Cookies Set Cookies
type Cookies []*http.Cookie

func (c *Cookies) IsEmpty() bool {
	return c == nil || len(*c) == 0
}

// Headers Set Header
type Headers map[string]string

func (h *Headers) IsEmpty() bool {
	return h == nil || len(*h) == 0
}

// FormData Use multipart/form-data
type File interface {
	Name() string
	Open() (io.ReadCloser, error)
}

type DiskFile string

func (f DiskFile) Name() string {
	return filepath.Base(string(f))
}

func (f DiskFile) Open() (io.ReadCloser, error) {
	return os.Open(string(f))
}

type MemoryFile struct {
	Filename string
	Reader   io.Reader
}

func (f *MemoryFile) Name() string {
	return f.Filename
}

func (f *MemoryFile) Open() (io.ReadCloser, error) {
	return io.NopCloser(f.Reader), nil
}

type FormData struct {
	Values map[string]any
	Files  map[string]File
}

func (f *FormData) IsEmpty() bool {
	return f == nil || (len(f.Values) == 0 && len(f.Files) == 0)
}

// FormData Use application/x-www-from-urlencoded
type FormKV map[string]string

func (f *FormKV) IsEmpty() bool {
	return f == nil || len(*f) == 0
}

// JSONPayload Use application/json
type JSONPayload interface {
	isJSONPayload()
	IsEmpty() bool
}

type JSONMap map[string]any

func (JSONMap) isJSONPayload() {}

func (j JSONMap) IsEmpty() bool {
	return len(j) == 0
}

type JSONArray []any

func (JSONArray) isJSONPayload() {}

func (l JSONArray) IsEmpty() bool {
	return len(l) == 0
}

type JSONStruct[T any] struct {
	V T
}

func (JSONStruct[T]) isJSONPayload() {}

func (s JSONStruct[T]) IsEmpty() bool {
	return false
}

func (s JSONStruct[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.V)
}

type JSONRaw []byte

func (JSONRaw) isJSONPayload() {}

func (r JSONRaw) IsEmpty() bool {
	return len(r) == 0
}

// Params Set Params
type Params map[string]string

func (p *Params) IsEmpty() bool {
	return p == nil || len(*p) == 0
}

// Retry
type OnRetry func(event RetryEvent)

type RetryDelay func(attempt int) time.Duration

type RetryPolicy func(resp *http.Response, err error) bool

// RateLimiter controls request pacing before a request is sent.
type RateLimiter interface {
	Wait(ctx context.Context) error
}

// Deprecated: Use new(expr) in Go 1.26+, e.g. new(true).
func PtrBool(b bool) *bool {
	return &b
}

// Deprecated: Use new(expr) in Go 1.26+, e.g. new(42).
func PtrInt(i int) *int {
	return &i
}

// Deprecated: Use new(expr) in Go 1.26+, e.g. new(42).
func PtrInt32(i int32) *int32 {
	return &i
}

// Deprecated: Use new(expr) in Go 1.26+, e.g. new(42).
func PtrInt64(i int64) *int64 {
	return &i
}

// Deprecated: Use new(expr) in Go 1.26+, e.g. new("hello").
func PtrString(s string) *string {
	return &s
}

func formValue(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int:
		return strconv.Itoa(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	}

	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
