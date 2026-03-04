package minireq

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Event string
	Data  string
	ID    string
	Retry int64
}

// NDJSONEvent represents a newline-delimited JSON event
type NDJSONEvent struct {
	Data  []byte
	codec JSONCodec
}

// Unmarshal parses the JSON data into the given value
func (e *NDJSONEvent) Unmarshal(v any) error {
	return e.codec.Unmarshal(e.Data, v)
}

// SSEReader reads Server-Sent Events from a stream
type SSEReader struct {
	reader *bufio.Reader
	closer io.Closer
}

// NDJSONReader reads newline-delimited JSON from a stream
type NDJSONReader struct {
	scanner *bufio.Scanner
	closer  io.Closer
	codec   JSONCodec
}

// NewSSEReader creates a new SSE reader
func NewSSEReader(r io.ReadCloser) *SSEReader {
	return &SSEReader{
		reader: bufio.NewReader(r),
		closer: r,
	}
}

// ReadEvent reads the next SSE event
func (r *SSEReader) ReadEvent() (*SSEEvent, error) {
	event := &SSEEvent{}
	var dataLines []string

	for {
		line, err := r.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF && len(line) == 0 {
				return nil, io.EOF
			}
			if err != io.EOF {
				return nil, err
			}
		}

		// Remove trailing newline
		line = bytes.TrimRight(line, "\r\n")

		// Empty line indicates end of event
		if len(line) == 0 {
			if len(dataLines) > 0 || event.Event != "" || event.ID != "" {
				event.Data = strings.Join(dataLines, "\n")
				return event, nil
			}
			continue
		}

		// Skip comments
		if line[0] == ':' {
			continue
		}

		// Parse field
		colonIdx := bytes.IndexByte(line, ':')
		var field, value string

		if colonIdx == -1 {
			field = string(line)
			value = ""
		} else {
			field = string(line[:colonIdx])
			value = string(bytes.TrimPrefix(line[colonIdx+1:], []byte{' '}))
		}

		switch field {
		case "event":
			event.Event = value
		case "data":
			dataLines = append(dataLines, value)
		case "id":
			event.ID = value
		case "retry":
			var retry int64
			fmt.Sscanf(value, "%d", &retry)
			event.Retry = retry
		}
	}
}

// Close closes the underlying reader
func (r *SSEReader) Close() error {
	if r.closer != nil {
		return r.closer.Close()
	}
	return nil
}

// Events returns a channel that yields SSE events
func (r *SSEReader) Events() <-chan SSEEvent {
	ch := make(chan SSEEvent)
	go func() {
		defer close(ch)
		for {
			event, err := r.ReadEvent()
			if err != nil {
				return
			}
			ch <- *event
		}
	}()
	return ch
}

// ReadEvent reads the next NDJSON event (one JSON object per line)
func (r *NDJSONReader) ReadEvent() (*NDJSONEvent, error) {
	for r.scanner.Scan() {
		line := bytes.TrimSpace(r.scanner.Bytes())
		// skip empty lines
		if len(line) == 0 {
			continue
		}
		// make a copy since scanner reuses the buffer
		data := make([]byte, len(line))
		copy(data, line)

		// validate JSON
		if !r.codec.Valid(data) {
			return nil, fmt.Errorf("invalid JSON: %s", string(data))
		}

		return &NDJSONEvent{
			Data:  data,
			codec: r.codec,
		}, nil
	}

	if err := r.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// Close closes the underlying reader
func (r *NDJSONReader) Close() error {
	if r.closer != nil {
		return r.closer.Close()
	}
	return nil
}

// Events returns a channel that yields NDJSON events
func (r *NDJSONReader) Events() <-chan NDJSONEvent {
	ch := make(chan NDJSONEvent)
	go func() {
		defer close(ch)
		for {
			event, err := r.ReadEvent()
			if err != nil {
				return
			}
			ch <- *event
		}
	}()
	return ch
}
