package minireq

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	URL "net/url"
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
	TLSHandshakeTimeout   int    // tls handshake timeout (seconds)
	HTTP2Enabled          bool   // enable HTTP/2
	MaxIdleConns          int    // maximum idle connections
	MaxIdleConnsPerHost   int    // maximum idle connections per host
	IdleConnTimeout       int    // idle connection timeout (seconds)
	ResponseHeaderTimeout int    // response header timeout (seconds)
}

type rateLimiterHolder struct {
	limiter RateLimiter
}

type HttpClient struct {
	Retry         *RetryConfig                   // retry
	jsonCodec     JSONCodec                      // per-client JSON codec
	transport     atomic.Pointer[http.Transport] // stores http.Transport
	cfg           atomic.Value                   // stores TransportConfig
	jar           atomic.Value                   // stores http.CookieJar
	limiter       atomic.Value                   // stores RateLimiter
	globalHeaders atomic.Value                   // stores headers
	timeout       atomic.Int64                   // stores timeout
	autoRedirect  atomic.Bool                    // stores autoRedirect
	multipartMode atomic.Int64                   // stores MultipartMode
}

type RequestOverride struct {
	Timeout              *int64 // seconds
	AutoRedirectDisabled *bool
}

type Request struct {
	Headers Headers
	Params  Params
	JSON    JSONPayload
	Form    FormKV
	Data    FormData
	Cookies Cookies
	Auth    Auth
}

func NewClient() *HttpClient {
	h := &HttpClient{
		Retry:     nil,
		jsonCodec: DefaultJSONCodec,
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
	h.globalHeaders.Store(map[string]string{})
	h.multipartMode.Store(int64(Buffered))

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

func (h *HttpClient) loadHeaders() map[string]string {
	if v := h.globalHeaders.Load(); v != nil {
		return v.(map[string]string)
	}
	return map[string]string{}
}

func (h *HttpClient) loadLimiter() RateLimiter {
	if v := h.limiter.Load(); v != nil {
		return v.(rateLimiterHolder).limiter
	}
	return nil
}

func buildMultipartBuffered(req *http.Request, f *FormData) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for k, v := range f.Values {
		if err := writer.WriteField(k, v); err != nil {
			return err
		}
	}

	for field, file := range f.Files {
		r, err := file.Open()
		if err != nil {
			return err
		}

		func() {
			defer r.Close()
			part, err := writer.CreateFormFile(field, file.Name())
			if err != nil {
				return
			}
			_, err = io.Copy(part, r)
		}()
	}

	if err := writer.Close(); err != nil {
		return err
	}

	buf := body.Bytes()

	req.Body = io.NopCloser(bytes.NewReader(buf))
	req.ContentLength = int64(len(buf))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf)), nil
	}

	return nil
}

func buildMultipartStream(req *http.Request, f *FormData) error {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	req.Body = pr
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ContentLength = -1 // chunked

	go func() {
		defer pw.Close()
		defer writer.Close()

		for k, v := range f.Values {
			if err := writer.WriteField(k, v); err != nil {
				pw.CloseWithError(err)
				return
			}
		}

		for field, file := range f.Files {
			r, err := file.Open()
			if err != nil {
				pw.CloseWithError(err)
				return
			}

			func() {
				defer r.Close()

				part, err := writer.CreateFormFile(field, file.Name())
				if err != nil {
					pw.CloseWithError(err)
					return
				}

				if _, err = io.Copy(part, r); err != nil {
					pw.CloseWithError(err)
					return
				}
			}()
		}
	}()

	return nil
}

func buildMultipart(req *http.Request, f *FormData, mode MultipartMode) error {
	switch mode {
	case Streaming:
		return buildMultipartStream(req, f)
	default:
		return buildMultipartBuffered(req, f)
	}
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

func (h *HttpClient) getOrCreateJar() (http.CookieJar, error) {
	if v := h.jar.Load(); v != nil {
		if jar, ok := v.(http.CookieJar); ok && jar != nil {
			return jar, nil
		}
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	h.jar.Store(jar)
	return jar, nil
}

type headerTransport struct {
	base    http.RoundTripper
	headers func() map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()

	for k, v := range t.headers() {
		if clone.Header.Get(k) == "" {
			clone.Header.Set(k, v)
		}
	}

	return t.base.RoundTrip(clone)
}

func (h *HttpClient) standardClient(timeout int64, autoRedirect bool) (*http.Client, error) {
	jar, err := h.getOrCreateJar()
	if err != nil {
		return nil, err
	}

	baseTransport := h.getTransport()

	client := &http.Client{
		Jar:     jar,
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &headerTransport{
			base:    baseTransport,
			headers: h.loadHeaders,
		},
	}

	if autoRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return client, nil
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

// reqOptions construct request options.
func reqOptions(request *http.Request, jsonCodec JSONCodec, multipartMode MultipartMode, opts ...any) (*http.Request, error) {
	for _, opt := range opts {
		switch t := opt.(type) {
		case *RetryConfig, *RequestOverride, context.Context:
			continue
		case Auth:
			request.SetBasicAuth(t.Username, t.Password)
		case Cookies:
			for _, c := range t {
				request.AddCookie(c)
			}
		case FormData:
			err := buildMultipart(request, &t, multipartMode)
			if err != nil {
				return nil, err
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
		case JSONPayload:
			if t == nil || t.IsEmpty() {
				return nil, fmt.Errorf("json payload is empty")
			}

			var bodyBytes []byte
			var err error

			switch v := t.(type) {
			case JSONRaw:
				bodyBytes = v
			default:
				bodyBytes, err = jsonCodec.Marshal(v)
				if err != nil {
					return nil, err
				}
			}

			request.Header.Set("Content-Type", "application/json")
			request.ContentLength = int64(len(bodyBytes))
			request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			request.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(bodyBytes)), nil
			}
		case Params:
			query := make(URL.Values)
			for k, v := range t {
				query.Add(k, v)
			}
			request.URL.RawQuery = query.Encode()
		case Request:
			var inner []any

			if !t.Headers.IsEmpty() {
				inner = append(inner, t.Headers)
			}
			if !t.Params.IsEmpty() {
				inner = append(inner, t.Params)
			}
			if !t.JSON.IsEmpty() {
				inner = append(inner, t.JSON)
			}
			if !t.Form.IsEmpty() {
				inner = append(inner, t.Form)
			}
			if !t.Data.IsEmpty() {
				inner = append(inner, t.Data)
			}
			if !t.Cookies.IsEmpty() {
				inner = append(inner, t.Cookies)
			}
			if !t.Auth.IsEmpty() {
				inner = append(inner, t.Auth)
			}

			return reqOptions(request, jsonCodec, multipartMode, inner...)
		}
	}
	return request, nil
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (h *HttpClient) doWithRetry(client *http.Client, request *http.Request, retryConfig *RetryConfig) (*http.Response, error) {
	var (
		resp        *http.Response
		err         error
		maxRetries  int
		retryPolicy = defaultRetryPolicy
		retryDelay  = defaultRetryDelay
		onRetry     = defaultOnRetry
	)

	if retryConfig != nil {
		maxRetries = max(retryConfig.MaxRetries, 0)

		if retryConfig.RetryPolicy != nil {
			retryPolicy = retryConfig.RetryPolicy
		}

		if retryConfig.RetryDelay != nil {
			retryDelay = retryConfig.RetryDelay
		}

		if retryConfig.OnRetry != nil {
			onRetry = retryConfig.OnRetry
		}
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check if context is cancelled before each attempt
		if err := request.Context().Err(); err != nil {
			return nil, err
		}
		if attempt > 0 {
			delay := nonNegativeDelay(retryDelay(attempt))
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
			if err := sleepWithContext(request.Context(), delay); err != nil {
				return nil, err
			}
		}

		if request.GetBody != nil {
			bodyCopy, err := request.GetBody()
			if err != nil {
				return nil, fmt.Errorf("failed to reset request body: %w", err)
			}
			request.Body = bodyCopy
		}

		resp, err = client.Do(request)

		shouldRetry := attempt < maxRetries && retryPolicy(resp, err)
		if !shouldRetry {
			break
		}

		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}
	return resp, err
}

func (h *HttpClient) SetMultipartMode(model MultipartMode) {
	h.multipartMode.Store(int64(model))
}

// SetRateLimiter sets a client-wide rate limiter.
// Passing nil disables rate limiting.
func (h *HttpClient) SetRateLimiter(limiter RateLimiter) {
	h.limiter.Store(rateLimiterHolder{limiter: limiter})
}

// SetHeader sets a global header on the client.
// Passing value == "" removes the header.
func (h *HttpClient) SetHeader(key, value string) {
	current := h.loadHeaders()
	newHeaders := make(map[string]string, len(current)+1)
	maps.Copy(newHeaders, current)
	if value == "" {
		delete(newHeaders, key)
	} else {
		newHeaders[key] = value
	}
	h.globalHeaders.Store(newHeaders)
}

// SetHeaders sets multiple global headers on the client.
// If a value is "", the header will be removed.
func (h *HttpClient) SetHeaders(headers map[string]string) {
	current := h.loadHeaders()
	newHeaders := make(map[string]string, len(current)+len(headers))
	maps.Copy(newHeaders, current)

	for k, v := range headers {
		if v == "" {
			delete(newHeaders, k)
		} else {
			newHeaders[k] = v
		}
	}

	h.globalHeaders.Store(newHeaders)
}

// SetJSONCodec sets the JSON codec for this client instance
// Pass nil to reset to the default encoding/json implementation
func (h *HttpClient) SetJSONCodec(codec JSONCodec) {
	if codec == nil {
		h.jsonCodec = DefaultJSONCodec
		return
	}
	h.jsonCodec = codec
}

// GetJSONCodec returns the JSON codec for this client instance
func (h *HttpClient) GetJSONCodec() JSONCodec {
	return h.jsonCodec
}

// StandardClient returns a configured net/http client for third-party integration.
func (h *HttpClient) StandardClient() (*http.Client, error) {
	timeout := int64(30)
	if v := h.timeout.Load(); v != 0 {
		timeout = v
	}

	return h.standardClient(timeout, h.autoRedirect.Load())
}

// Transport returns the underlying shared transport instance.
// StandardClient wraps this transport to inject global headers.
func (h *HttpClient) Transport() *http.Transport {
	return h.getTransport()
}

// CookieJar returns the client's shared cookie jar.
func (h *HttpClient) CookieJar() (http.CookieJar, error) {
	return h.getOrCreateJar()
}

// SetCookieJar replaces the client's shared cookie jar.
// Passing nil resets it to a fresh default jar.
func (h *HttpClient) SetCookieJar(jar http.CookieJar) {
	if jar == nil {
		jar, _ = cookiejar.New(nil)
	}
	h.jar.Store(jar)
}

// SetTimeout Set timeout in seconds
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

// SetIdleConnTimeout Set Idle Connection Timeout (seconds)
func (h *HttpClient) SetIdleConnTimeout(seconds int) {
	cfg := h.loadConfig()
	cfg.IdleConnTimeout = seconds
	h.storeConfig(cfg)
	if old := h.clearTransport(); old != nil {
		old.CloseIdleConnections()
	}
}

// SetResponseHeaderTimeout Set Response Header Timeout (seconds)
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

// SetTLSHandshakeTimeout Set TLS Handshake Timeout (seconds)
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
	var retryConfig *RetryConfig
	var ctx context.Context

	multipartMode := MultipartMode(h.multipartMode.Load())

	for _, opt := range opts {
		switch t := opt.(type) {
		case *RequestOverride:
			override = t
		case *RetryConfig:
			retryConfig = t
		case context.Context:
			ctx = t
		}
	}

	if ctx == nil {
		ctx = context.Background()
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

	// apply global headers first (allow per-request headers to override)
	for k, v := range h.loadHeaders() {
		request.Header.Set(k, v)
	}

	request, err = reqOptions(request, h.jsonCodec, multipartMode, opts...)
	if err != nil {
		return nil, err
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

	client, err := h.standardClient(timeout, autoRedirect)
	if err != nil {
		return nil, err
	}

	// retry
	effectiveRetryConfig := h.Retry
	if retryConfig != nil {
		effectiveRetryConfig = retryConfig
	}

	if multipartMode == Streaming && effectiveRetryConfig != nil && effectiveRetryConfig.MaxRetries > 0 {
		return nil, fmt.Errorf("streaming multipart does not support retry")
	}

	if limiter := h.loadLimiter(); limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	// Send Data
	reqForSend := request.Clone(ctx)
	if request.GetBody != nil {
		if rb, err := request.GetBody(); err == nil {
			reqForSend.Body = rb
			reqForSend.GetBody = request.GetBody
		}
	}
	response, err := h.doWithRetry(client, reqForSend, effectiveRetryConfig)
	if err != nil {
		return nil, err
	}
	miniRes := new(MiniResponse)
	miniRes.Request = request
	miniRes.Response = response
	miniRes.jsonCodec = h.jsonCodec
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
