package minireq

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type MiniResponse struct {
	Request   *http.Request
	Response  *http.Response
	jsonCodec JSONCodec
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
	err = res.jsonCodec.Unmarshal(rawData, &jsonData)
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
	err = res.jsonCodec.UnmarshalNumber(rawData, &jsonData)
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

func (res *MiniResponse) ReadSSE() (*SSEReader, error) {
	if res.bodyCache != nil {
		return NewSSEReader(io.NopCloser(bytes.NewReader(res.bodyCache))), nil
	}
	if res.Response == nil || res.Response.Body == nil {
		return nil, fmt.Errorf("response or response body is nil")
	}

	body := res.Response.Body
	res.Response.Body = nil
	return NewSSEReader(body), nil
}

// ReadNDJSON returns an NDJSONReader for reading newline-delimited JSON streams
func (res *MiniResponse) ReadNDJSON() (*NDJSONReader, error) {
	if res.bodyCache != nil {
		return &NDJSONReader{
			scanner: bufio.NewScanner(io.NopCloser(bytes.NewReader(res.bodyCache))),
			closer:  io.NopCloser(bytes.NewReader(res.bodyCache)),
			codec:   res.jsonCodec,
		}, nil
	}
	if res.Response == nil || res.Response.Body == nil {
		return nil, fmt.Errorf("response or response body is nil")
	}

	body := res.Response.Body
	res.Response.Body = nil
	return &NDJSONReader{
		scanner: bufio.NewScanner(body),
		closer:  body,
		codec:   res.jsonCodec,
	}, nil
}

// ReadStreamAuto automatically detects content type and returns SSEReader or NDJSONReader
// Returns (sseReader, nil, nil) for SSE, or (nil, ndjsonReader, nil) for NDJSON
func (res *MiniResponse) ReadStreamAuto() (*SSEReader, *NDJSONReader, error) {
	contentType := ""
	if res.Response != nil {
		contentType = res.Response.Header.Get("Content-Type")
	}

	if strings.Contains(contentType, "application/x-ndjson") ||
		strings.Contains(contentType, "application/jsonl") ||
		strings.Contains(contentType, "application/json-lines") {
		reader, err := res.ReadNDJSON()
		return nil, reader, err
	}

	// default to SSE
	reader, err := res.ReadSSE()
	return reader, nil, err
}

func (res *MiniResponse) StreamSSE(callback func(event SSEEvent) error) error {
	reader, err := res.ReadSSE()
	if err != nil {
		return err
	}
	defer reader.Close()

	for {
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if err := callback(*event); err != nil {
			return err
		}
	}
}

// StreamNDJSON iterates over NDJSON events and calls the callback for each one
func (res *MiniResponse) StreamNDJSON(callback func(event NDJSONEvent) error) error {
	reader, err := res.ReadNDJSON()
	if err != nil {
		return err
	}
	defer reader.Close()

	for {
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if err := callback(*event); err != nil {
			return err
		}
	}
}

// StreamNDJSONUnmarshal iterates over NDJSON events, unmarshals each into a new instance of the given type, and calls callback
func StreamNDJSONUnmarshal[T any](res *MiniResponse, callback func(item T) error) error {
	reader, err := res.ReadNDJSON()
	if err != nil {
		return err
	}
	defer reader.Close()

	for {
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		var item T
		if err := res.jsonCodec.Unmarshal(event.Data, &item); err != nil {
			return fmt.Errorf("failed to unmarshal NDJSON: %w", err)
		}

		if err := callback(item); err != nil {
			return err
		}
	}
}
