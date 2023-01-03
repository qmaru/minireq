package minireq

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type MiniResponse struct {
	Request  *http.Request
	Response *http.Response
}

// RawData bytes data
func (res *MiniResponse) RawData() ([]byte, error) {
	body := res.Response.Body
	defer body.Close()

	bodyData, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return bodyData, nil
}

// RawJSON JSON data
func (res *MiniResponse) RawJSON() (interface{}, error) {
	var jsonData interface{}
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
func (res *MiniResponse) RawNumJSON() (interface{}, error) {
	var jsonData interface{}

	rawData, err := res.RawData()
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(strings.NewReader(string(rawData)))
	dec.UseNumber()
	err = dec.Decode(&jsonData)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}
