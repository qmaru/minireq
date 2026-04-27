package minireq

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	Values map[string]string
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

type JSONObject map[string]any

func (JSONObject) isJSONPayload() {}

func (j JSONObject) IsEmpty() bool {
	return len(j) == 0
}

type JSONList []any

func (JSONList) isJSONPayload() {}

func (l JSONList) IsEmpty() bool {
	return len(l) == 0
}

type JSONStruct[T any] struct {
	V T
}

func (JSONStruct[T]) isJSONPayload() {}

func (s JSONStruct[T]) IsEmpty() bool {
	return false
}

type RawJSON []byte

func (RawJSON) isJSONPayload() {}

func (r RawJSON) IsEmpty() bool {
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
