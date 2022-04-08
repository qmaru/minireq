package minireq

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// DefaultVer 版本号
const DefaultVer = "1.1.0"

// DefaultUA 默认 User-Agent
const DefaultUA = "MiniRequest/" + DefaultVer

// Headers 设置 Header
type Headers map[string]string

// Params 设置 Params
type Params map[string]string

// JSONData 设置 JSON Data
type JSONData map[string]interface{}

// FormData 设置 Form Data
type FormData map[string]string

// FileData 设置 File Data
type FileData map[string]interface{}

// Auth 设置 HTTP Basic Auth
type Auth []string

// MiniRequest 提供基本 HTTP 请求
type MiniRequest struct {
	Request *http.Request
	Header  *http.Header
	Client  *http.Client
}

// MiniResponse response
type MiniResponse struct {
	RawRes *http.Response
	RawReq *MiniRequest
}

// setFile 上传文件处理
//	可以一次上传多个文件
func (mr *MiniRequest) setFile(files map[string]interface{}) error {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	for filed, filelist := range files {
		if reflect.TypeOf(filelist).String() == "map[string]string" {
			for k, v := range filelist.(map[string]string) {
				bodyWriter.WriteField(k, v)
			}
		} else {
			for _, file := range filelist.([]string) {
				// 打开文件
				f, err := os.Open(file)
				if err != nil {
					return err
				}
				defer f.Close()
				// 创建表单
				fileWriter, err := bodyWriter.CreateFormFile(filed, filepath.Base(file))
				if err != nil {
					return err
				}
				// 写入缓存
				_, err = io.Copy(fileWriter, f)
				if err != nil {
					return err
				}
			}
		}
	}

	err := bodyWriter.Close()
	if err != nil {
		return err
	}
	mr.Request.Header.Set("Content-Type", bodyWriter.FormDataContentType())
	mr.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBuf.Bytes()))
	return nil
}

// setOptions 根据参数类型设置值
//	1.设置 Header
//	2.设置 Auth
//	3.设置 Params
//	4.设置 Data
func (mr *MiniRequest) setOption(opt interface{}) error {
	switch t := opt.(type) {
	case Headers:
		for k, v := range t {
			mr.Request.Header.Set(k, v)
		}
	case Auth:
		mr.Request.SetBasicAuth(t[0], t[1])
	case Params:
		q := mr.Request.URL.Query()
		for k, v := range t {
			q.Add(k, v)
		}
		mr.Request.URL.RawQuery = q.Encode()
	case JSONData:
		bytesData, err := json.Marshal(t)
		if err != nil {
			return err
		}
		reader := bytes.NewReader(bytesData)
		mr.Request.Header.Set("Content-Type", "application/json")
		mr.Request.Body = ioutil.NopCloser(reader)
	case FormData:
		p := mr.Request.URL.Query()
		for k, v := range t {
			p.Add(k, v)
		}
		reader := strings.NewReader(p.Encode())
		mr.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		mr.Request.Body = ioutil.NopCloser(reader)
	case FileData:
		err := mr.setFile(t)
		if err != nil {
			return err
		}
	}
	return nil
}

// Requests 设置默认的HTTP客户端
//	1.默认的UserAgent: MiniRequest
//	2.自动保存Cookies
//	3.超时时间30秒
func Requests() (*MiniRequest, error) {
	req := new(MiniRequest)

	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	req.Request = &http.Request{
		Method: "GET",
		Header: make(http.Header),
	}
	req.Header = &req.Request.Header
	req.Request.Header.Set("User-Agent", DefaultUA)

	req.Client = &http.Client{
		Jar:     cookieJar,
		Timeout: 30 * time.Second,
	}
	return req, nil
}

// NoRedirect 取消自动重定向
func (mr *MiniRequest) NoRedirect(s bool) {
	if s {
		mr.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		mr.Client.CheckRedirect = nil
	}
}

// NoCookieJar 关闭cookiejar
func (mr *MiniRequest) NoCookieJar(s bool) error {
	if s {
		mr.Client.Jar = nil
	} else {
		cookieJar, err := cookiejar.New(nil)
		if err != nil {
			return err
		}
		mr.Client.Jar = cookieJar
	}
	return nil
}

// TimeOut 设置超时时间 默认30s
func (mr *MiniRequest) TimeOut(t int) {
	mr.Client.Timeout = time.Duration(t) * time.Second
}

// Proxy 设置Socks5代理
//	eg: 127.0.0.1:1080
func (mr *MiniRequest) Proxy(proxyURL string) error {
	dialer, err := proxy.SOCKS5("tcp", proxyURL,
		nil,
		&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	)
	if err != nil {
		return err
	}

	mr.Client.Transport = &http.Transport{
		Proxy:               nil,
		Dial:                dialer.Dial,
		TLSHandshakeTimeout: 30 * time.Second,
	}
	return nil
}

// SetCookies 设置 Cookies
//	建议先关闭 NoCookieJar(true) 关闭cookiejar
func (mr *MiniRequest) SetCookies(cookies []*http.Cookie) {
	oldCookies := mr.Request.Cookies()
	if len(oldCookies) != 0 {
		delete(mr.Request.Header, "Cookie")
	}
	for _, c := range cookies {
		mr.Request.AddCookie(c)
	}
}

// Get GET请求
//	1.获取原始的 Response
//	2.获取原始的 Request
func (mr *MiniRequest) Get(rawURL string, opts ...interface{}) (response *MiniResponse, err error) {
	parseURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	// 重置 Cookies
	delete(mr.Request.Header, "Cookie")

	mr.Request.Method = "GET"
	mr.Request.URL = parseURL

	for _, opt := range opts {
		mr.setOption(opt)
	}

	rawRes, err := mr.Client.Do(mr.Request)
	if err != nil {
		return nil, err
	}

	response = &MiniResponse{}
	response.RawRes = rawRes
	response.RawReq = mr
	return response, nil
}

// Post POST请求
//	1.获取原始的 Response
//	2.获取原始的 Request
func (mr *MiniRequest) Post(rawURL string, opts ...interface{}) (response *MiniResponse, err error) {
	parseURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	// 重置 Cookies
	delete(mr.Request.Header, "Cookie")

	mr.Request.Method = "POST"
	mr.Request.URL = parseURL

	for _, opt := range opts {
		mr.setOption(opt)
	}

	rawRes, err := mr.Client.Do(mr.Request)
	if err != nil {
		return nil, err
	}
	response = &MiniResponse{}
	response.RawRes = rawRes
	response.RawReq = mr
	return response, nil
}

// RawData 获取Response的Body
func (res *MiniResponse) RawData() ([]byte, error) {
	defer res.RawRes.Body.Close()

	body := res.RawRes.Body
	bodyData, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return bodyData, nil
}

// RawJSON 获取Response的JSON数据
func (res *MiniResponse) RawJSON() (interface{}, error) {
	var jsonData interface{}
	rawData, err := res.RawData()
	if err != nil {
		return nil, err
	}
	json.Unmarshal(rawData, &jsonData)
	return jsonData, nil
}

// RawNumJSON 获取Response的JSON数据(保留整型大数)
func (res *MiniResponse) RawNumJSON() (interface{}, error) {
	var jsonData interface{}

	rawData, err := res.RawData()
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(strings.NewReader(string(rawData)))
	dec.UseNumber()
	dec.Decode(&jsonData)
	return jsonData, nil
}
