package minireq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type MiniResponse struct {
	Request   *http.Request
	Response  *http.Response
	bodyCache []byte
}

// Close Close response body
func (res *MiniResponse) Close() error {
	if res.Response == nil || res.Response.Body == nil {
		return nil
	}

	defer func() {
		_ = res.Response.Body.Close()
		res.Response.Body = nil
	}()

	if res.bodyCache != nil {
		return nil
	}

	_, _ = io.Copy(io.Discard, res.Response.Body)
	return nil
}

// RawData bytes data
func (res *MiniResponse) RawData() ([]byte, error) {
	if res.bodyCache != nil {
		return res.bodyCache, nil
	}

	if res.Response == nil || res.Response.Body == nil {
		return nil, fmt.Errorf("response or response body is nil")
	}

	body := res.Response.Body
	defer func() {
		_ = body.Close()
		res.Response.Body = nil
	}()

	bodyData, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	res.bodyCache = bodyData
	return bodyData, nil
}

// RawJSON JSON data
func (res *MiniResponse) RawJSON() (any, error) {
	var jsonData any
	rawData, err := res.RawData()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(rawData, &jsonData)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

// RawNumJSON JSON data with real number
func (res *MiniResponse) RawNumJSON() (any, error) {
	var jsonData any

	rawData, err := res.RawData()
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(rawData))
	dec.UseNumber()
	err = dec.Decode(&jsonData)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func (res *MiniResponse) ReadStream() (io.ReadCloser, error) {
	if res.bodyCache != nil {
		return io.NopCloser(bytes.NewReader(res.bodyCache)), nil
	}
	if res.Response == nil || res.Response.Body == nil {
		return nil, fmt.Errorf("response or response body is nil")
	}

	body := res.Response.Body
	res.Response.Body = nil
	return body, nil
}
