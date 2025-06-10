package minireq

import (
	"io"
	"net/http"
	"time"
)

// DefaultVer Library version
const DefaultVer = "2.0.0"

// DefaultUA Default User-Agent
const DefaultUA = "MiniRequest/" + DefaultVer

type FileInMemory struct {
	Filename string
	Reader   io.Reader
}

// Auth Set HTTP Basic Auth
type Auth []string

// Cookies Set Cookies
type Cookies []*http.Cookie

// FormData Use multipart/form-data
type FormData struct {
	Values map[string]string
	Files  map[string]any
}

// FormData Use application/x-www-from-urlencoded
type FormKV map[string]string

// Headers Set Header
type Headers map[string]string

// JSONData Use application/json
type JSONData map[string]any

// Params Set Params
type Params map[string]string

// Retry
type OnRetry func(event RetryEvent)

type RetryDelay func(attempt int) time.Duration

type RetryPolicy func(resp *http.Response, err error) bool
