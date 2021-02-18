package minireq

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
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
const DefaultVer = "1.0"

// DefaultUA 默认 User-Agent
const DefaultUA = "MiniRequest/" + DefaultVer

// Headers 设置 Header
type Headers map[string]string

// Params 设置 Params
type Params map[string]string

// JSONData 设置 JSON Data
type JSONData map[string]string

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
	RawRes  *http.Response
	RawReq  *MiniRequest
	rawData []byte
	rawJSON interface{}
}

// setFile 上传文件处理
//	可以一次上传多个文件
func (mr *MiniRequest) setFile(files map[string]interface{}) {
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
					log.Panic("Open File Error", err)
				}
				defer f.Close()
				// 创建表单
				fileWriter, err := bodyWriter.CreateFormFile(filed, filepath.Base(file))
				if err != nil {
					log.Panic("File Buffer Error", err)
				}
				// 写入缓存
				_, err = io.Copy(fileWriter, f)
				if err != nil {
					log.Panic("Write Data Error", err)
				}
			}
		}
	}

	err := bodyWriter.Close()
	if err != nil {
		log.Panic("Buffer Close Error")
	}
	mr.Request.Header.Set("Content-Type", bodyWriter.FormDataContentType())
	mr.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBuf.Bytes()))
}

// setOptions 根据参数类型设置值
//	1.设置 Header
//	2.设置 Auth
//	3.设置 Params
//	4.设置 Data
func (mr *MiniRequest) setOption(opt interface{}) {
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
		bytesData, _ := json.Marshal(t)
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
		mr.setFile(t)
	}
}

// Requests 设置默认的HTTP客户端
//	1.默认的UserAgent: MiniRequest
//	2.自动保存Cookies
//	3.超时时间30秒
func Requests() *MiniRequest {
	req := new(MiniRequest)

	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
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
	return req
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
func (mr *MiniRequest) NoCookieJar(s bool) {
	if s {
		mr.Client.Jar = nil
	} else {
		cookieJar, err := cookiejar.New(nil)
		if err != nil {
			panic(err)
		}
		mr.Client.Jar = cookieJar
	}
}

// TimeOut 设置超时时间 默认30s
func (mr *MiniRequest) TimeOut(t int) {
	mr.Client.Timeout = time.Duration(t) * time.Second
}

// Proxy 设置Socks5代理
//	eg: 127.0.0.1:1080
func (mr *MiniRequest) Proxy(proxyURL string) {
	dialer, err := proxy.SOCKS5("tcp", proxyURL,
		nil,
		&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	)
	if err != nil {
		log.Panic(" [S5 Proxy Error]: ", err)
	}

	mr.Client.Transport = &http.Transport{
		Proxy:               nil,
		Dial:                dialer.Dial,
		TLSHandshakeTimeout: 30 * time.Second,
	}
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
func (mr *MiniRequest) Get(rawURL string, opts ...interface{}) (response *MiniResponse) {
	parseURL, err := url.Parse(rawURL)
	if err != nil {
		log.Panic(err)
	}

	mr.Request.Method = "GET"
	mr.Request.URL = parseURL

	for _, opt := range opts {
		mr.setOption(opt)
	}

	rawRes, err := mr.Client.Do(mr.Request)
	if err != nil {
		log.Panic(" [Get - Response Error]: ", err)
	}

	response = &MiniResponse{}
	response.RawRes = rawRes
	response.RawReq = mr
	return response
}

// Post POST请求
//	1.获取原始的 Response
//	2.获取原始的 Request
func (mr *MiniRequest) Post(rawURL string, opts ...interface{}) (response *MiniResponse) {
	parseURL, err := url.Parse(rawURL)
	if err != nil {
		log.Panic(err)
	}

	mr.Request.Method = "POST"
	mr.Request.URL = parseURL

	for _, opt := range opts {
		mr.setOption(opt)
	}

	rawRes, err := mr.Client.Do(mr.Request)
	if err != nil {
		log.Panic(" [Post - Response Error]: ", err)
	}
	response = &MiniResponse{}
	response.RawRes = rawRes
	response.RawReq = mr
	return response
}

// RawData 获取Response的Body
func (res *MiniResponse) RawData() []byte {
	defer res.RawRes.Body.Close()

	var err error
	body := res.RawRes.Body
	res.rawData, err = ioutil.ReadAll(body)
	if err != nil {
		log.Panic(" [Content - Body Error]: ", err)
	}
	return res.rawData
}

// RawJSON 获取Response的JSON数据
func (res *MiniResponse) RawJSON() interface{} {
	var jsonData interface{}
	res.RawData()
	rawData := res.rawData
	json.Unmarshal(rawData, &jsonData)
	res.rawJSON = jsonData
	return res.rawJSON
}

// RawNumJSON 获取Response的JSON数据(保留整型大数)
func (res *MiniResponse) RawNumJSON() interface{} {
	var jsonData interface{}
	res.RawData()
	rawData := res.rawData

	dec := json.NewDecoder(strings.NewReader(string(rawData)))
	dec.UseNumber()
    dec.Decode(&jsonData)

	res.rawJSON = jsonData
	return res.rawJSON
}
