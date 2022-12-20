package minireq

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
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
	"time"

	"golang.org/x/net/proxy"
)

type HttpClient struct {
	Method        string // Request Method
	NoRedirect    bool   // Turn off automatic redirection
	Socks5Address string // Set socks5 proxy
	SkipVerify    bool   // Skip verify
	Timeout       int    // Request timeout
}

func NewClient() *HttpClient {
	client := new(HttpClient)
	client.Timeout = 30
	return client
}

// setProxy Set socks5 proxy
func setProxy(address string) (proxy.Dialer, error) {
	addRule := regexp.MustCompile(`^((2(5[0-5]|[0-4]\d))|[0-1]?\d{1,2})(\.((2(5[0-5]|[0-4]\d))|[0-1]?\d{1,2})){3}:\d{1,5}$`)
	if !addRule.MatchString(address) {
		return nil, errors.New("address is error")
	}

	dialer, err := proxy.SOCKS5("tcp", address, nil,
		&net.Dialer{
			Timeout:   time.Duration(30) * time.Second,
			KeepAlive: time.Duration(30) * time.Second,
		},
	)
	if err != nil {
		return nil, err
	}
	return dialer, nil
}

// reqOptions construct a body
func reqOptions(request *http.Request, opts interface{}) (*http.Request, error) {
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
			for k, v := range files {
				f, err := os.Open(v)
				if err != nil {
					return nil, err
				}
				defer f.Close()
				// create form data
				fileWriter, err := bodyWriter.CreateFormFile(k, filepath.Base(v))
				if err != nil {
					return nil, err
				}
				_, err = io.Copy(fileWriter, f)
				if err != nil {
					return nil, err
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

// Request Universal client
func (h *HttpClient) Request(url string, opts ...interface{}) (*miniResponse, error) {
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
	request.Header.Set("User-Agent", DefaultUA)
	for _, opt := range opts {
		request, err = reqOptions(request, opt)
		if err != nil {
			return nil, err
		}
	}
	// Make Client
	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Jar:     cookieJar,
		Timeout: time.Duration(h.Timeout) * time.Second,
	}
	clientTransport := new(http.Transport)
	// allow Redirect
	if h.NoRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		client.CheckRedirect = nil
	}
	// allow proxy
	if h.Socks5Address != "" {
		dialer, err := setProxy(h.Socks5Address)
		if err != nil {
			return nil, err
		}
		dialContext := func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.Dial(network, address)
		}
		clientTransport.Proxy = nil
		clientTransport.DialContext = dialContext
		clientTransport.TLSHandshakeTimeout = time.Duration(30) * time.Second
	}
	if h.SkipVerify {
		clientTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client.Transport = clientTransport
	// Send Data
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	miniRes := new(miniResponse)
	miniRes.Request = request
	miniRes.Response = response
	return miniRes, nil
}

func (h *HttpClient) Get(url string, opts ...interface{}) (*miniResponse, error) {
	h.Method = "GET"
	return h.Request(url, opts...)
}

func (h *HttpClient) Post(url string, opts ...interface{}) (*miniResponse, error) {
	h.Method = "POST"
	return h.Request(url, opts...)
}

func (h *HttpClient) Put(url string, opts ...interface{}) (*miniResponse, error) {
	h.Method = "PUT"
	return h.Request(url, opts...)
}

func (h *HttpClient) Patch(url string, opts ...interface{}) (*miniResponse, error) {
	h.Method = "PATCH"
	return h.Request(url, opts...)
}

func (h *HttpClient) Delete(url string, opts ...interface{}) (*miniResponse, error) {
	h.Method = "DELETE"
	return h.Request(url, opts...)
}
