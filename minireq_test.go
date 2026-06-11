package minireq

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"testing"
)

const HTTPBIN string = "https://httpbun.com"

func TestGet(t *testing.T) {
	client := NewClient()
	params := Params{"foo": "bar"}
	res, err := client.Get(HTTPBIN+"/get", params)
	if err != nil {
		t.Error(err)
	} else {
		rawData, err := res.RawJSON()
		if err != nil {
			t.Error(err)
		} else {
			jsonData := rawData.(map[string]any)
			args := jsonData["args"].(map[string]any)
			if _, ok := args["foo"]; ok {
				t.Log("succeed")
			} else {
				t.Error("failed")
			}
		}
	}
}

func TestProxy(t *testing.T) {
	client := NewClient()

	t.Log("s5 proxy")
	client.SetSocks5Proxy("127.0.0.1:1080")
	res, err := client.Get(HTTPBIN + "/ip")
	if err != nil {
		t.Error(err)
	} else {
		rawData, err := res.RawJSON()
		if err != nil {
			t.Error(err)
		} else {
			jsonData := rawData.(map[string]any)
			ip := jsonData["origin"]
			t.Log(ip)
		}
	}

	t.Log("http proxy")
	client.SetHttpProxyURL("http://127.0.0.1:1080")
	res, err = client.Get(HTTPBIN + "/ip")
	if err != nil {
		t.Error(err)
	} else {
		rawData, err := res.RawJSON()
		if err != nil {
			t.Error(err)
		} else {
			jsonData := rawData.(map[string]any)
			ip := jsonData["origin"]
			t.Log(ip)
		}
	}
}

func TestAuth(t *testing.T) {
	client := NewClient()
	auth := Auth{"postman", "password"}
	res, err := client.Get(HTTPBIN+"/basic-auth/postman/password", auth)
	if err != nil {
		t.Error(err)
	} else {
		rawData, err := res.RawJSON()
		if err != nil {
			t.Error(err)
		} else {
			jsonData := rawData.(map[string]any)
			if _, ok := jsonData["authenticated"]; ok {
				t.Log("succeed")
			} else {
				t.Error("failed")
			}
		}
	}
}

func TestPostURL(t *testing.T) {
	client := NewClient()
	data := FormKV{"foo": "bar"}
	res, err := client.Post(HTTPBIN+"/post", data)
	if err != nil {
		t.Error(err)
	} else {
		rawData, err := res.RawJSON()
		if err != nil {
			t.Error(err)
		} else {
			jsonData := rawData.(map[string]any)
			form := jsonData["form"].(map[string]any)
			headers := jsonData["headers"].(map[string]any)
			contentType := headers["Content-Type"].(string)
			if _, ok := form["foo"]; ok && contentType == "application/x-www-form-urlencoded" {
				t.Log("succeed")
			} else {
				t.Error("failed")
			}
		}
	}
}

func TestPostData(t *testing.T) {
	client := NewClient()

	imageBase64 := "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"
	imageByte, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		t.Fatal(err)
	}

	diskFile := FormData{
		Values: map[string]string{"foo": "bar"},
		Files:  map[string]File{"file1": DiskFile("go.mod")},
	}

	memoryFile := FormData{
		Values: map[string]string{"foo": "bar"},
		Files: map[string]File{
			"file1": &MemoryFile{
				Filename: "file1_by_memory",
				Reader:   bytes.NewReader(imageByte),
			},
		},
	}

	diskRes, err := client.Post(HTTPBIN+"/post", diskFile)
	if err != nil {
		t.Fatal(err)
	}

	diskJson, err := diskRes.RawJSON()
	if err != nil {
		t.Fatal(err)
	}

	diskData := diskJson.(map[string]any)

	diskForm := diskData["form"].(map[string]any)
	diskFiles := diskData["files"].(map[string]any)
	diskHeaders := diskData["headers"].(map[string]any)
	diskContentType := diskHeaders["Content-Type"].(string)

	_, diskOk1 := diskForm["foo"]
	_, diskOk2 := diskFiles["file1"]
	diskOk3 := strings.Contains(diskContentType, "multipart/form-data")

	if diskOk1 && diskOk2 && diskOk3 {
		t.Log("disk file upload succeed")
	} else {
		t.Error("disk file upload failed")
	}

	memoryRes, err := client.Post(HTTPBIN+"/post", memoryFile)
	if err != nil {
		t.Fatal(err)
	}

	memoryJson, err := memoryRes.RawJSON()
	if err != nil {
		t.Fatal(err)
	}

	memoryData := memoryJson.(map[string]any)

	memoryForm := memoryData["form"].(map[string]any)
	memoryFiles := memoryData["files"].(map[string]any)
	memoryHeaders := memoryData["headers"].(map[string]any)
	memoryContentType := memoryHeaders["Content-Type"].(string)

	_, memoryOk1 := memoryForm["foo"]
	_, memoryOk2 := memoryFiles["file1"]
	memoryOk3 := strings.Contains(memoryContentType, "multipart/form-data")

	if memoryOk1 && memoryOk2 && memoryOk3 {
		t.Log("memory file upload succeed")
	} else {
		t.Error("memory file upload failed")
	}

	client.SetMultipartMode(Streaming)
	memoryStreamRes, err := client.Post(HTTPBIN+"/post", memoryFile)
	if err != nil {
		t.Fatal(err)
	}

	memoryStreamJson, err := memoryStreamRes.RawJSON()
	if err != nil {
		t.Fatal(err)
	}

	memoryStreamData := memoryStreamJson.(map[string]any)
	memoryStreamHeaders := memoryStreamData["headers"].(map[string]any)
	meooryTransferEncoding := memoryStreamHeaders["Transfer-Encoding"].([]any)
	if len(meooryTransferEncoding) != 0 {
		t.Logf("streaming multipart upload succeed: %s\n", meooryTransferEncoding)
	} else {
		t.Error("streaming multipart upload failed")
	}
}

func TestPostJSON(t *testing.T) {
	client := NewClient()
	data := JSONMap{"foo": "bar"}
	res, err := client.Post(HTTPBIN+"/post", data)
	if err != nil {
		t.Error(err)
	} else {
		rawData, err := res.RawJSON()
		if err != nil {
			t.Error(err)
		} else {
			jsonData := rawData.(map[string]any)
			json := jsonData["json"].(map[string]any)
			headers := jsonData["headers"].(map[string]any)
			contentType := headers["Content-Type"].(string)
			if _, ok := json["foo"]; ok && contentType == "application/json" {
				t.Log("succeed")
			} else {
				t.Error("failed")
			}
		}
	}
}

func TestSetHeader(t *testing.T) {
	client := NewClient()
	client.SetHeader("X-Global-Header", "test-value")
	res, err := client.Get(HTTPBIN + "/get")
	if err != nil {
		t.Fatal(err)
	}

	rawData, err := res.RawJSON()
	if err != nil {
		t.Fatal(err)
	}

	jsonData := rawData.(map[string]any)
	headers := jsonData["headers"].(map[string]any)
	if headers["X-Global-Header"] != "test-value" {
		t.Fatalf("expected X-Global-Header=test-value, got %v", headers["X-Global-Header"])
	}

	// per-request override should win
	res2, err := client.Get(HTTPBIN+"/get", Headers{"X-Global-Header": "override-value"})
	if err != nil {
		t.Fatal(err)
	}

	rawData2, err := res2.RawJSON()
	if err != nil {
		t.Fatal(err)
	}

	jsonData2 := rawData2.(map[string]any)
	headers2 := jsonData2["headers"].(map[string]any)
	if headers2["X-Global-Header"] != "override-value" {
		t.Fatalf("expected X-Global-Header=override-value, got %v", headers2["X-Global-Header"])
	}
}

func TestRetry(t *testing.T) {
	testCodePool := []int{
		200, 201, 204,
		408, 429,
		500, 502, 503, 504,
	}

	rpmPool := []int{5, 10, 20, 50, 100, 1000}

	maxRetries := 3
	rpm := rpmPool[rand.Intn(len(rpmPool))]
	errCode := []int{500, 502, 503, 504, 408, 429}

	retryDelayFn, err := RetryExponentialDelayFromRPM(rpm, 0)
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient()
	client.Retry = NewRetryDefaultConfig()

	client.Retry.MaxRetries = maxRetries
	client.Retry.RetryDelay = retryDelayFn
	client.Retry.RetryPolicy = RetryPolicyWithStatusCodes(errCode...)

	client.Retry.OnRetry = func(event RetryEvent) {
		status := event.Status
		t.Logf("[retry] #%d | rpm=%d | status=%d | err=%v | delay=%s\n",
			event.Attempt, rpm, status, event.Err, event.Delay)
	}

	testCode := testCodePool[rand.Intn(len(testCodePool))]
	url := fmt.Sprintf("%s/status/%d", HTTPBIN, testCode)
	t.Logf("Request URL: %s\n", url)
	res, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	statusCode := res.Response.StatusCode
	t.Logf("Latest code: %d\n", statusCode)
}

func TestRetry2(t *testing.T) {
	client := NewClient()
	client.SetTimeout(30)

	t.Log("Retry policy 1")
	retryConfig1 := &RetryConfig{
		MaxRetries:  2,
		RetryPolicy: RetryPolicyWithStatusCodes(500, 502, 503, 504),
		RetryDelay:  RetryFixedDelay(100 * time.Millisecond),
		OnRetry: func(event RetryEvent) {
			t.Logf("[Retry1] Attempt #%d | Status: %d | Delay: %s",
				event.Attempt, event.Status, event.Delay)
		},
	}

	res1, err := client.Get(HTTPBIN+"/status/500", retryConfig1)
	if err != nil {
		t.Logf("Request 1 error: %v", err)
	} else {
		t.Logf("Request 1 final status: %d", res1.Response.StatusCode)
	}

	t.Log("Retry policy 2")
	retryConfig2 := &RetryConfig{
		MaxRetries:  5,
		RetryPolicy: RetryPolicyWithStatusCodes(408, 429),
		RetryDelay:  RetryExponentialDelay(50*time.Millisecond, 0.1),
		OnRetry: func(event RetryEvent) {
			t.Logf("[Retry2] Attempt #%d | Status: %d | Delay: %s",
				event.Attempt, event.Status, event.Delay)
		},
	}

	res2, err := client.Get(HTTPBIN+"/status/429", retryConfig2)
	if err != nil {
		t.Logf("Request 2 error: %v", err)
	} else {
		t.Logf("Request 2 final status: %d", res2.Response.StatusCode)
	}

	t.Log("Retry policy 3")
	res3, err := client.Get(HTTPBIN + "/status/200")
	if err != nil {
		t.Logf("Request 3 error: %v", err)
	} else {
		t.Logf("Request 3 final status: %d", res3.Response.StatusCode)
	}

	t.Log("Retry policy 4: Concurrent requests with different retry configs")
	var wg sync.WaitGroup
	results := make(chan string, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()
		retryConf := &RetryConfig{
			MaxRetries:  2,
			RetryPolicy: RetryPolicyWithStatusCodes(503),
			RetryDelay:  RetryFixedDelay(50 * time.Millisecond),
		}
		res, err := client.Get(HTTPBIN+"/status/503", retryConf)
		if err != nil {
			results <- fmt.Sprintf("Concurrent1 error: %v", err)
		} else {
			results <- fmt.Sprintf("Concurrent1 status: %d", res.Response.StatusCode)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		retryConf := &RetryConfig{
			MaxRetries:  1,
			RetryPolicy: RetryPolicyWithStatusCodes(502),
			RetryDelay:  RetryFixedDelay(75 * time.Millisecond),
		}
		res, err := client.Get(HTTPBIN+"/status/502", retryConf)
		if err != nil {
			results <- fmt.Sprintf("Concurrent2 error: %v", err)
		} else {
			results <- fmt.Sprintf("Concurrent2 status: %d", res.Response.StatusCode)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		res, err := client.Get(HTTPBIN + "/status/200")
		if err != nil {
			results <- fmt.Sprintf("Concurrent3 error: %v", err)
		} else {
			results <- fmt.Sprintf("Concurrent3 status: %d", res.Response.StatusCode)
		}
	}()

	wg.Wait()
	close(results)

	for result := range results {
		t.Log(result)
	}

	t.Log("completed all concurrent requests")
}

func TestRetry3(t *testing.T) {
	client := NewClient()
	client.SetTimeout(30)

	bodyText := "retry-final-body-still-readable"
	bodyBase64 := base64.StdEncoding.EncodeToString([]byte(bodyText))
	url := fmt.Sprintf("%s/mix/s=500/b64=%s", HTTPBIN, bodyBase64)

	retryCount := 0
	res, err := client.Get(url, &RetryConfig{
		MaxRetries:  2,
		RetryPolicy: RetryPolicyWithStatusCode(500),
		RetryDelay:  RetryNoDelay(),
		OnRetry: func(event RetryEvent) {
			retryCount++
			t.Logf("[Retry3] Attempt #%d | Status: %d | Delay: %s",
				event.Attempt, event.Status, event.Delay)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer res.Close()

	if retryCount == 0 {
		t.Fatal("expected retry to happen, but it did not")
	}

	if res.Response.StatusCode != 500 {
		t.Fatalf("expected final status 500, got %d", res.Response.StatusCode)
	}

	body, err := io.ReadAll(res.Response.Body)
	if err != nil {
		t.Fatalf("expected final response body to remain readable, got error: %v", err)
	}

	if string(body) != bodyText {
		t.Fatalf("expected final body %q, got %q", bodyText, string(body))
	}

	t.Logf("Retry3 final status: %d, retries: %d, body: %q", res.Response.StatusCode, retryCount, string(body))
}

func TestStreamBody(t *testing.T) {
	client := NewClient()
	res, err := client.Get(HTTPBIN + "/bytes/90")
	if err != nil {
		t.Fatal(err)
	}

	if res.bodyCache != nil {
		t.Fatalf("expected no bodyCache before stream, got len=%d", len(res.bodyCache))
	}

	stream, err := res.ReadStream()
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var buf bytes.Buffer
	n, err := io.Copy(&buf, stream)
	if err != nil {
		t.Fatal(err)
	}

	if n != 90 {
		t.Fatalf("expected 90 bytes, got %d", n)
	} else {
		t.Logf("succeed, got %d bytes", n)
	}

	if res.Response != nil && res.Response.Body != nil {
		t.Fatalf("expected res.Response.Body == nil after ReadStream ownership transfer")
	}
}

func TestAnySet(t *testing.T) {
	client := NewClient()

	startT := time.Now()
	client.SetTimeout(15)
	t.Log("set timeout 5s")
	res, err := client.Get(HTTPBIN + "/delay/3")
	elapsed := time.Since(startT)
	t.Logf("elapsed time: %s\n", elapsed)

	if err != nil {
		t.Fatal(err)
	} else {
		statusCode := res.Response.StatusCode
		t.Log(statusCode)
	}

	transportAddr1 := fmt.Sprintf("%p", client.transport.Load())
	t.Logf("First transport address: %s\n", transportAddr1)

	t.Log("set insecure")
	client.SetInsecure(true)
	res, err = client.Get("https://expired.badssl.com/")
	if err != nil {
		t.Error(err)
	} else {
		statusCode := res.Response.StatusCode
		t.Log(statusCode)
	}

	transportAddr2 := fmt.Sprintf("%p", client.transport.Load())
	t.Logf("After request 1 transport address: %s\n", transportAddr2)

	t.Log("set redirect")
	client.DisableAutoRedirect(true)
	res, err = client.Get(HTTPBIN + "/redirect/3")
	if err != nil {
		t.Error(err)
	} else {
		statusCode := res.Response.StatusCode
		t.Log(statusCode)
	}

	transportAddr3 := fmt.Sprintf("%p", client.transport.Load())
	t.Logf("After all override requests, transport address: %s\n", transportAddr3)

	if transportAddr1 == transportAddr3 {
		t.Log("transport reused as expected")
	} else {
		t.Error("transport address changed unexpectedly")
	}
}

func TestOverride(t *testing.T) {
	client := NewClient()

	res, err := client.Get(HTTPBIN+"/delay/1", &RequestOverride{Timeout: PtrInt64(3)})
	if err != nil {
		t.Fatal(err)
	} else {
		statusCode := res.Response.StatusCode
		t.Log(statusCode)
	}
	transportAddr1 := fmt.Sprintf("%p", client.transport.Load())
	t.Logf("First transport address: %s\n", transportAddr1)

	res, err = client.Get(HTTPBIN+"/redirect/1", &RequestOverride{AutoRedirectDisabled: PtrBool(true)})
	if err != nil {
		t.Error(err)
	} else {
		statusCode := res.Response.StatusCode
		t.Logf("redirect disable status: %d", statusCode)
	}
	transportAddr2 := fmt.Sprintf("%p", client.transport.Load())
	t.Logf("After request 1 transport address: %s\n", transportAddr2)

	if transportAddr1 == transportAddr2 {
		t.Log("transport reused as expected")
	} else {
		t.Error("transport address changed unexpectedly")
	}
}

func TestClientReuse(t *testing.T) {
	client := NewClient()
	client.SetTimeout(15)

	const workers = 10
	results := make(chan string, workers)
	errs := make(chan error, workers)

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			res, err := client.Get(HTTPBIN + "/get")
			if err != nil {
				errs <- err
				return
			}
			defer res.Response.Body.Close()

			_, _ = io.Copy(io.Discard, res.Response.Body)
			results <- fmt.Sprintf("%p", client.transport.Load())
		}(i)
	}

	wg.Wait()
	close(results)
	close(errs)

	if len(errs) > 0 {
		for e := range errs {
			t.Fatalf("request failed: %v", e)
		}
	}

	addrs := make(map[string]int)
	for a := range results {
		addrs[a]++
	}

	if len(addrs) != 1 {
		t.Errorf("expected single transport reused, got %d distinct: %v", len(addrs), addrs)
	} else {
		for k, v := range addrs {
			t.Logf("transport reused %d times: %s", v, k)
		}
	}
}

func TestRace(t *testing.T) {
	c := NewClient()

	const (
		requestGoroutines = 8
		setterGoroutines  = 4
		loopsPerWorker    = 8
	)

	var wg sync.WaitGroup

	wg.Add(requestGoroutines)
	for i := 0; i < requestGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < loopsPerWorker; j++ {
				params := Params{"foo": strconv.Itoa(id), "n": strconv.Itoa(j)}
				_, err := c.Get(HTTPBIN+"/get", params)
				if err != nil {
					t.Logf("request err: %v", err)
				}
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Add(setterGoroutines)
	for i := 0; i < setterGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < loopsPerWorker; j++ {
				switch j % 7 {
				case 1:
					c.SetHttpProxyURL("")
				case 2:
					c.SetMaxIdleConns(50 + (id+j)%50)
				case 3:
					c.SetMaxIdleConnsPerHost(5 + (id+j)%20)
				case 4:
					c.SetIdleConnTimeout(20 + (id+j)%20)
				case 5:
					c.SetHTTP2(j%2 == 0)
				case 6:
					c.SetInsecure(j%2 == 0)
				}
				c.SetTimeout(int64(3 + (id+j)%10))
				time.Sleep(6 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
}

func TestSSE(t *testing.T) {
	client := NewClient()
	res, err := client.Get(HTTPBIN + "/sse")
	if err != nil {
		t.Fatal(err)
	}

	eventCount := 0
	err = res.StreamSSE(func(event SSEEvent) error {
		fmt.Printf("Event: %s, Data: %s\n", event.Event, event.Data)
		eventCount++
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Total events received: %d\n", eventCount)
}

func TestContextCancel(t *testing.T) {
	timeout := 2 * time.Second
	t.Log("Testing request cancellation with context timeout")
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err := client.Get(HTTPBIN+"/delay/5", ctx)
	if err != nil {
		t.Logf("Request cancelled as expected: %v, cancel with timeout %.0f seconds", err, timeout.Seconds())
	} else {
		t.Error("Expected request to be cancelled, but it succeeded")
	}
}

func TestRatelimit(t *testing.T) {
	client := NewClient()

	rpmLimit, err := RateLimitFromRPM(30)
	if err != nil {
		t.Fatalf("Failed to create RPM rate limiter: %v", err)
	}

	client.SetRateLimiter(rpmLimit)
	client.Get(HTTPBIN + "/get")
}
