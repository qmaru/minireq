package minireq

import "net/http"

// DefaultVer Library version
const DefaultVer = "2.0.0"

// DefaultUA Default User-Agent
const DefaultUA = "MiniRequest/" + DefaultVer

// Auth Set HTTP Basic Auth
type Auth []string

// Cookies Set Cookies
type Cookies []*http.Cookie

// FormData Use multipart/form-data
type FormData struct {
	Values map[string]string
	Files  map[string]string
}

// FormData Use application/x-www-from-urlencoded
type FormKV map[string]string

// Headers Set Header
type Headers map[string]string

// JSONData Use application/json
type JSONData map[string]interface{}

// Params Set Params
type Params map[string]string
