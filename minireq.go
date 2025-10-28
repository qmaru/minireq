package minireq

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	URL "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
)

type transportConfig struct {
	Insecure              bool   // allow insecure request
	HttpProxyAddress      string // http proxy addr
	Socks5ProxyAddress    string // socks5 proxy addr
	TLSHandshakeTimeout   int    // tls handshake timeout
	HTTP2Enabled          bool   // enable HTTP/2
	MaxIdleConns          int    // maximum idle connections
	MaxIdleConnsPerHost   int    // maximum idle connections per host
	IdleConnTimeout       int    // seconds
	ResponseHeaderTimeout int    // seconds
}

type HttpClient struct {
	Retry        *RetryConfig                   // retry
	transport    atomic.Pointer[http.Transport] // stores http.Transport
	jar          atomic.Value                   // stores http.CookieJar
	cfg          atomic.Value                   // stores TransportConfig
	timeout      atomic.Int64                   // stores int
	autoRedirect atomic.Bool                    // stores bool
}

type RequestOverride struct {
	Timeout              *int64
	AutoRedirectDisabled *bool
}

func PtrBool(b bool) *bool {
	return &b
}

func PtrInt(i int) *int {
	return &i
}

func PtrInt32(i int32) *int32 {
	return &i
}

func PtrInt64(i int64) *int64 {
	return &i
}

func PtrString(s string) *string {
	return &s
}

func NewClient() *HttpClient {
	h := &HttpClient{
		Retry: nil,
	}

	// initialize defaults
	defaultCfg := transportConfig{
		Socks5ProxyAddress:  "",
		TLSHandshakeTimeout: 30,
		HTTP2Enabled:        true,
	}

	h.cfg.Store(defaultCfg)
	h.timeout.Store(30)
	h.autoRedirect.Store(false)

	if jar, err := cookiejar.New(nil); err == nil {
		h.jar.Store(jar)
	}

	return h
}

func (h *HttpClient) loadConfig() transportConfig {
	if v := h.cfg.Load(); v != nil {
		return v.(transportConfig)
	}
	return transportConfig{
		Socks5ProxyAddress:  "",
		TLSHandshakeTimeout: 30,
		HTTP2Enabled:        true,
	}
}

func (h *HttpClient) storeConfig(c transportConfig) {
	h.cfg.Store(c)
}

func (h *HttpClient) getTransport() *http.Transport {
	if t := h.transport.Load(); t != nil {
		return t
	}

	cfg := h.loadConfig()
	clientTransport := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: cfg.Insecure},
		TLSHandshakeTimeout: time.Duration(cfg.TLSHandshakeTimeout) * time.Second,
		ForceAttemptHTTP2:   cfg.HTTP2Enabled,
	}

	if cfg.HttpProxyAddress != "" {
		if pu, err := URL.Parse(cfg.HttpProxyAddress); err == nil {
			clientTransport.Proxy = http.ProxyURL(pu)
		}
	}

	if cfg.Socks5ProxyAddress != "" {
		dialer, err := setS5Proxy(cfg.Socks5ProxyAddress)
		if err == nil {
			clientTransport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialer.Dial(network, address)
			}
		}
	}

	if cfg.MaxIdleConns > 0 {
		clientTransport.MaxIdleConns = cfg.MaxIdleConns
	}

	if cfg.MaxIdleConnsPerHost > 0 {
		clientTransport.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	}

	if cfg.IdleConnTimeout > 0 {
		clientTransport.IdleConnTimeout = time.Duration(cfg.IdleConnTimeout) * time.Second
	}

	if cfg.ResponseHeaderTimeout > 0 {
		clientTransport.ResponseHeaderTimeout = time.Duration(cfg.ResponseHeaderTimeout) * time.Second
	}

	if loaded := h.transport.Swap(clientTransport); loaded != nil {
		clientTransport.CloseIdleConnections()
		return loaded
	}

	return clientTransport
}

func (h *HttpClient) clearTransport() *http.Transport {
	old := h.transport.Swap(nil)
	return old
}

// setS5Proxy Set socks5 proxy
func setS5Proxy(address string) (proxy.Dialer, error) {
	addRule := regexp.MustCompile(`^((2(5[0-5]|[0-4]\d))|[0-1]?\d{1,2})(\.((2(5[0-5]|[0-4]\d))|[0-1]?\d{1,2})){3}:\d{1,5}$`)
	if !addRule.MatchString(address) {
		return nil, fmt.Errorf("address is error")
	}

	dialer, err := proxy.SOCKS5("tcp", address, nil,
		&net.Dialer{
			Timeout:   time.Duration(30) * time.Second,
			KeepAlive: time.Duration(30) * time.Second,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set proxy: %s", err.Error())
	}
	return dialer, nil
}

// reqOptions construct a body
func reqOptions(request *http.Request, opts any) (*http.Request, error) {
	switch t := opts.(type) {
	case Auth:
		request.SetBasicAuth(t[0], t[1])
	case Cookies:
		for _, c := range t {
			request.AddCookie(c)
		}
	case FormData:
		bodyBuf := &bytes.Buffer{}
		bodyWriter := multipart.NewWriter(bodyBuf)

		// Fill parameters
		if t.Values != nil {
			values := t.Values
			for k, v := range values {
				err := bodyWriter.WriteField(k, v)
				if err != nil {
					return nil, err
				}
			}
		}

		// Fill files
		if t.Files != nil {
			files := t.Files
			for fieldName, fileObj := range files {
				switch f := fileObj.(type) {
				case string:
					file, err := os.Open(f)
					if err != nil {
						return nil, err
					}
					defer file.Close()
					// create form data
					fileWriter, err := bodyWriter.CreateFormFile(fieldName, filepath.Base(f))
					if err != nil {
						return nil, err
					}
					if _, err = io.Copy(fileWriter, file); err != nil {
						return nil, err
					}
				case *FileInMemory:
					fileWriter, err := bodyWriter.CreateFormFile(fieldName, f.Filename)
					if err != nil {
						return nil, err
					}
					if _, err := io.Copy(fileWriter, f.Reader); err != nil {
						return nil, err
					}
				default:
					return nil, fmt.Errorf("unsupported file type for field %s", fieldName)
				}
			}
		}

		err := bodyWriter.Close()
		if err != nil {
			return nil, err
		}

		reader := bytes.NewBuffer(bodyBuf.Bytes())
		buf := reader.Bytes()

		request.ContentLength = int64(reader.Len())
		request.Header.Set("Content-Type", bodyWriter.FormDataContentType())
		request.Body = io.NopCloser(reader)
		request.GetBody = func() (io.ReadCloser, error) {
			r := bytes.NewReader(buf)
			return io.NopCloser(r), nil
		}
	case FormKV:
		query := make(URL.Values)
		for k, v := range t {
			query.Add(k, v)
		}
		reader := strings.NewReader(query.Encode())
		snapshot := *reader

		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.ContentLength = int64(reader.Len())
		request.Body = io.NopCloser(reader)
		request.GetBody = func() (io.ReadCloser, error) {
			r := snapshot
			return io.NopCloser(&r), nil
		}
	case Headers:
		for k, v := range t {
			request.Header.Set(k, v)
		}
	case JSONData:
		jsonByte, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		reader := bytes.NewReader(jsonByte)
		snapshot := *reader

		request.Header.Set("Content-Type", "application/json")
		request.ContentLength = int64(reader.Len())
		request.Body = io.NopCloser(reader)
		request.GetBody = func() (io.ReadCloser, error) {
			r := snapshot
			return io.NopCloser(&r), nil
		}
	case Params:
		query := make(URL.Values)
		for k, v := range t {
			query.Add(k, v)
		}
		request.URL.RawQuery = query.Encode()
	}
	return request, nil
}

func (h *HttpClient) doWithRetry(client *http.Client, request *http.Request) (*http.Response, error) {
	var (
		resp        *http.Response
		err         error
		maxRetries  int
		retryPolicy = defaultRetryPolicy
		retryDelay  = defaultRetryDelay
		onRetry     = defaultOnRetry
	)

	if h.Retry != nil {
		maxRetries = h.Retry.MaxRetries

		if h.Retry.RetryPolicy != nil {
			retryPolicy = h.Retry.RetryPolicy
		}

		if h.Retry.RetryDelay != nil {
			retryDelay = h.Retry.RetryDelay
		}

		if h.Retry.OnRetry != nil {
			onRetry = h.Retry.OnRetry
		}
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryDelay(attempt)
			if onRetry != nil {
				status := 0
				if resp != nil {
					status = resp.StatusCode
				}
				onRetry(RetryEvent{
					Attempt: attempt,
					Status:  status,
					Err:     err,
					Delay:   delay,
				})
			}
			time.Sleep(delay)
		}

		if request.GetBody != nil {
			bodyCopy, err := request.GetBody()
			if err != nil {
				return nil, fmt.Errorf("failed to reset request body: %w", err)
			}
			request.Body = bodyCopy
		}

		resp, err = client.Do(request)

		if !retryPolicy(resp, err) {
			break
		}

		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}
	return resp, err
}

// SetTimeout Set timeout
func (h *HttpClient) SetTimeout(i int64) {
	h.timeout.Store(i)
}

// DisableAutoRedirect Disable Redirect
func (h *HttpClient) DisableAutoRedirect(enabled bool) {
	h.autoRedirect.Store(enabled)
}

// SetSocks5Proxy Set socks5 proxy
func (h *HttpClient) SetSocks5Proxy(addr string) {
	cfg := h.loadConfig()
	cfg.Socks5ProxyAddress = addr
	if addr != "" {
		cfg.HttpProxyAddress = ""
	}
	h.storeConfig(cfg)
	if old := h.clearTransport(); old != nil {
		old.CloseIdleConnections()
	}
}

// SetHttpProxyURL Set http proxy
func (h *HttpClient) SetHttpProxyURL(proxyURL string) {
	cfg := h.loadConfig()
	cfg.HttpProxyAddress = proxyURL
	if proxyURL != "" {
		cfg.Socks5ProxyAddress = ""
	}
	h.storeConfig(cfg)
	if old := h.clearTransport(); old != nil {
		old.CloseIdleConnections()
	}
}

// SetMaxIdleConns Set Max Idle Connections
func (h *HttpClient) SetMaxIdleConns(n int) {
	cfg := h.loadConfig()
	cfg.MaxIdleConns = n
	h.storeConfig(cfg)
	if old := h.clearTransport(); old != nil {
		old.CloseIdleConnections()
	}
}

// SetMaxIdleConnsPerHost Set Max Idle Connections Per Host
func (h *HttpClient) SetMaxIdleConnsPerHost(n int) {
	cfg := h.loadConfig()
	cfg.MaxIdleConnsPerHost = n
	h.storeConfig(cfg)
	if old := h.clearTransport(); old != nil {
		old.CloseIdleConnections()
	}
}

// SetIdleConnTimeout Set Idle Connection Timeout
func (h *HttpClient) SetIdleConnTimeout(seconds int) {
	cfg := h.loadConfig()
	cfg.IdleConnTimeout = seconds
	h.storeConfig(cfg)
	if old := h.clearTransport(); old != nil {
		old.CloseIdleConnections()
	}
}

// SetResponseHeaderTimeout Set Response Header Timeout
func (h *HttpClient) SetResponseHeaderTimeout(seconds int) {
	cfg := h.loadConfig()
	cfg.ResponseHeaderTimeout = seconds
	h.storeConfig(cfg)
	if old := h.clearTransport(); old != nil {
		old.CloseIdleConnections()
	}
}

// SetHTTP2 Enable HTTP2
func (h *HttpClient) SetHTTP2(enabled bool) {
	cfg := h.loadConfig()
	cfg.HTTP2Enabled = enabled
	h.storeConfig(cfg)
	if old := h.clearTransport(); old != nil {
		old.CloseIdleConnections()
	}
}

// SetInsecure Allow Insecure
func (h *HttpClient) SetInsecure(enabled bool) {
	cfg := h.loadConfig()
	cfg.Insecure = enabled
	h.storeConfig(cfg)
	if old := h.clearTransport(); old != nil {
		old.CloseIdleConnections()
	}
}

// SetTLSHandshakeTimeout Set TLS Handshake Timeout
func (h *HttpClient) SetTLSHandshakeTimeout(t int) {
	cfg := h.loadConfig()
	cfg.TLSHandshakeTimeout = t
	h.storeConfig(cfg)
	if old := h.clearTransport(); old != nil {
		old.CloseIdleConnections()
	}
}

// Request Universal client
func (h *HttpClient) RequestWithMethod(method, url string, opts ...any) (*MiniResponse, error) {
	var err error
	var override *RequestOverride

	finalOpts := []any{}
	for _, opt := range opts {
		if ro, ok := opt.(*RequestOverride); ok {
			override = ro
		} else {
			finalOpts = append(finalOpts, opt)
		}
	}

	// Make URL
	parseURL, err := URL.Parse(url)
	if err != nil {
		return nil, err
	}

	// Make Request
	request := &http.Request{
		URL:    parseURL,
		Method: method,
		Header: make(http.Header),
	}

	for _, opt := range finalOpts {
		request, err = reqOptions(request, opt)
		if err != nil {
			return nil, err
		}
	}

	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", DefaultUA)
	}

	timeout := int64(30)
	if v := h.timeout.Load(); v != 0 {
		timeout = v
	}
	autoRedirect := h.autoRedirect.Load()

	if override != nil {
		if override.Timeout != nil {
			timeout = *override.Timeout
		}
		if override.AutoRedirectDisabled != nil {
			autoRedirect = *override.AutoRedirectDisabled
		}
	}

	transport := h.getTransport()

	// cookie jar
	var jar http.CookieJar
	if v := h.jar.Load(); v != nil {
		jar = v.(http.CookieJar)
	}

	if jar == nil {
		j, err := cookiejar.New(nil)
		if err != nil {
			return nil, err
		}
		h.jar.Store(j)
		jar = j
	}

	client := &http.Client{
		Jar:       jar,
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: transport,
	}

	// disable redirect
	if autoRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// Send Data
	reqForSend := request.Clone(context.Background())
	if request.GetBody != nil {
		if rb, err := request.GetBody(); err == nil {
			reqForSend.Body = rb
			reqForSend.GetBody = request.GetBody
		}
	}
	response, err := h.doWithRetry(client, reqForSend)
	if err != nil {
		return nil, err
	}
	miniRes := new(MiniResponse)
	miniRes.Request = request
	miniRes.Response = response
	return miniRes, nil
}

func (h *HttpClient) Get(url string, opts ...any) (*MiniResponse, error) {
	return h.RequestWithMethod("GET", url, opts...)
}

func (h *HttpClient) Post(url string, opts ...any) (*MiniResponse, error) {
	return h.RequestWithMethod("POST", url, opts...)
}

func (h *HttpClient) Put(url string, opts ...any) (*MiniResponse, error) {
	return h.RequestWithMethod("PUT", url, opts...)
}

func (h *HttpClient) Patch(url string, opts ...any) (*MiniResponse, error) {
	return h.RequestWithMethod("PATCH", url, opts...)
}

func (h *HttpClient) Delete(url string, opts ...any) (*MiniResponse, error) {
	return h.RequestWithMethod("DELETE", url, opts...)
}

func (h *HttpClient) Connect(url string, opts ...any) (*MiniResponse, error) {
	return h.RequestWithMethod("CONNECT", url, opts...)
}

func (h *HttpClient) Head(url string, opts ...any) (*MiniResponse, error) {
	return h.RequestWithMethod("HEAD", url, opts...)
}

func (h *HttpClient) Options(url string, opts ...any) (*MiniResponse, error) {
	return h.RequestWithMethod("OPTIONS", url, opts...)
}

func (h *HttpClient) Trace(url string, opts ...any) (*MiniResponse, error) {
	return h.RequestWithMethod("TRACE", url, opts...)
}
