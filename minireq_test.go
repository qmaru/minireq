package minireq

import (
	"bytes"
	"encoding/base64"
	"strings"

	"testing"
)

const HTTPBIN string = "https://httpbin.org"

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
	client.SetProxy("127.0.0.1:1080")
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

	data1 := FormData{
		Values: map[string]string{"foo": "bar"},
		Files:  map[string]any{"file1": "go.mod"},
	}

	data2 := FormData{
		Values: map[string]string{"foo": "bar"},
		Files: map[string]any{
			"file1": &FileInMemory{
				Filename: "file1_by_memory",
				Reader:   bytes.NewReader(imageByte),
			},
		},
	}

	res1, err := client.Post(HTTPBIN+"/post", data1)
	if err != nil {
		t.Fatal(err)
	}

	rawData1, err := res1.RawJSON()
	if err != nil {
		t.Fatal(err)
	}

	jsonData1 := rawData1.(map[string]any)

	form1 := jsonData1["form"].(map[string]any)
	files1 := jsonData1["files"].(map[string]any)
	headers1 := jsonData1["headers"].(map[string]any)
	contentType1 := headers1["Content-Type"].(string)

	_, resp1ok1 := form1["foo"]
	_, resp1ok2 := files1["file1"]
	resp1ok3 := strings.Contains(contentType1, "multipart/form-data")

	if resp1ok1 && resp1ok2 && resp1ok3 {
		t.Log("data1 succeed")
	} else {
		t.Error("data1 failed")
	}

	res2, err := client.Post(HTTPBIN+"/post", data2)
	if err != nil {
		t.Fatal(err)
	}

	rawData2, err := res2.RawJSON()
	if err != nil {
		t.Fatal(err)
	}

	jsonData2 := rawData2.(map[string]any)

	form2 := jsonData2["form"].(map[string]any)
	files2 := jsonData2["files"].(map[string]any)
	headers2 := jsonData2["headers"].(map[string]any)
	contentType2 := headers2["Content-Type"].(string)

	_, resp2ok1 := form2["foo"]
	_, resp2ok2 := files2["file1"]
	resp2ok3 := strings.Contains(contentType2, "multipart/form-data")

	if resp2ok1 && resp2ok2 && resp2ok3 {
		t.Log("data2 succeed")
	} else {
		t.Error("data2 failed")
	}
}

func TestPostJSON(t *testing.T) {
	client := NewClient()
	data := JSONData{"foo": "bar"}
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

func TestAnySet(t *testing.T) {
	client := NewClient()
	client.SetTimeout(5)
	t.Log("set timeout 5s")
	res, err := client.Get(HTTPBIN + "/delay/3")
	if err != nil {
		t.Fatal(err)
	} else {
		statusCode := res.Response.StatusCode
		t.Log(statusCode)
	}

	t.Log("set insecure")
	client.SetInsecure(false)
	res, err = client.Get("https://expired.badssl.com/")
	if err != nil {
		t.Error(err)
	} else {
		statusCode := res.Response.StatusCode
		t.Log(statusCode)
	}

	t.Log("set redirect")
	client.SetAutoRedirectDisable(true)
	res, err = client.Get(HTTPBIN + "/redirect/3")
	if err != nil {
		t.Error(err)
	} else {
		statusCode := res.Response.StatusCode
		t.Log(statusCode)
	}
}
