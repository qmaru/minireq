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
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type TransportConfig struct {
	Insecure            bool   // allow insecure request
	Socks5Address       string // socks5 proxy addr
	TLSHandshakeTimeout int    // tls handshake timeout
}

type HttpClient struct {
	Method              string          // Request Method
	Timeout             int             // request timeout
	AutoRedirectDisable bool            // automatic redirection
	Insecure            bool            // allow insecure request
	Socks5Address       string          // socks5 proxy addr
	TLSHandshakeTimeout int             // tls handshake timeout
	TransportConfig     TransportConfig // transport
	Retry               *RetryConfig    // retry
	transport           *http.Transport
}

var transportPool = sync.Pool{
	New: func() any {
		return &http.Transport{}
	},
}

func NewClient() *HttpClient {
	return &HttpClient{
		Timeout:             30,
		AutoRedirectDisable: false,
		Insecure:            false,
		Socks5Address:       "",
		TLSHandshakeTimeout: 30,
	}
}

func (h *HttpClient) getTransport(cfg TransportConfig) *http.Transport {
	if h.transport == nil || h.TransportConfig != cfg {
		clientTransport := transportPool.Get().(*http.Transport)
		// transport config
		clientTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: cfg.Insecure}
		clientTransport.TLSHandshakeTimeout = time.Duration(cfg.TLSHandshakeTimeout) * time.Second

		if cfg.Socks5Address != "" {
			dialer, err := setProxy(cfg.Socks5Address)
			if err == nil {
				clientTransport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
					return dialer.Dial(network, address)
				}
			}
		}
		h.TransportConfig = cfg
		h.transport = clientTransport
	}

	return h.transport
}

// setProxy Set socks5 proxy
func setProxy(address string) (proxy.Dialer, error) {
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
func (h *HttpClient) SetTimeout(t int) {
	h.Timeout = t
}

// SetProxy Set socks5 proxy
func (h *HttpClient) SetProxy(addr string) {
	h.getTransport(TransportConfig{Socks5Address: addr})
}

// SetInsecure Allow Insecure
func (h *HttpClient) SetInsecure(t bool) {
	h.getTransport(TransportConfig{Insecure: t})
}

// SetAutoRedirectDisable Disable Redirect
func (h *HttpClient) SetAutoRedirectDisable(t bool) {
	h.AutoRedirectDisable = t
}

// Request Universal client
func (h *HttpClient) Request(url string, opts ...any) (*MiniResponse, error) {
	var err error
	// Make URL
	parseURL, err := URL.Parse(url)
	if err != nil {
		return nil, err
	}
	// Make Request
	request := &http.Request{
		URL:    parseURL,
		Method: h.Method,
		Header: make(http.Header),
	}

	for _, opt := range opts {
		request, err = reqOptions(request, opt)
		if err != nil {
			return nil, err
		}
	}

	if request.Header.Get("user-agent") == "" {
		request.Header.Set("User-Agent", DefaultUA)
	}

	// Make Client
	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	transport := h.getTransport(h.TransportConfig)

	client := &http.Client{
		Jar:       cookieJar,
		Timeout:   time.Duration(h.Timeout) * time.Second,
		Transport: transport,
	}

	// disable redirect
	if h.AutoRedirectDisable {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		client.CheckRedirect = nil
	}

	// Send Data
	response, err := h.doWithRetry(client, request)
	if err != nil {
		return nil, err
	}
	miniRes := new(MiniResponse)
	miniRes.Request = request
	miniRes.Response = response
	return miniRes, nil
}

func (h *HttpClient) doRequest(method, url string, opts ...any) (*MiniResponse, error) {
	h.Method = method
	return h.Request(url, opts...)
}

func (h *HttpClient) Get(url string, opts ...any) (*MiniResponse, error) {
	return h.doRequest("GET", url, opts...)
}

func (h *HttpClient) Post(url string, opts ...any) (*MiniResponse, error) {
	return h.doRequest("POST", url, opts...)
}

func (h *HttpClient) Put(url string, opts ...any) (*MiniResponse, error) {
	return h.doRequest("PUT", url, opts...)
}

func (h *HttpClient) Patch(url string, opts ...any) (*MiniResponse, error) {
	return h.doRequest("PATCH", url, opts...)
}

func (h *HttpClient) Delete(url string, opts ...any) (*MiniResponse, error) {
	return h.doRequest("DELETE", url, opts...)
}

func (h *HttpClient) Connect(url string, opts ...any) (*MiniResponse, error) {
	return h.doRequest("CONNECT", url, opts...)
}

func (h *HttpClient) Head(url string, opts ...any) (*MiniResponse, error) {
	return h.doRequest("HEAD", url, opts...)
}

func (h *HttpClient) Options(url string, opts ...any) (*MiniResponse, error) {
	return h.doRequest("OPTIONS", url, opts...)
}

func (h *HttpClient) Trace(url string, opts ...any) (*MiniResponse, error) {
	return h.doRequest("TRACE", url, opts...)
}
