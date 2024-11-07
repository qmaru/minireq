package minireq

import (
	"strings"

	"testing"
)

const HTTPBIN string = "https://httpbin.org/"

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
	data := FormData{
		Values: map[string]string{"foo": "bar"},
		Files:  map[string]string{"file1": "go.mod"},
	}
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
			files := jsonData["files"].(map[string]any)
			headers := jsonData["headers"].(map[string]any)
			contentType := headers["Content-Type"].(string)

			_, ok1 := form["foo"]
			_, ok2 := files["file1"]
			ok3 := strings.Contains(contentType, "multipart/form-data")
			if ok1 && ok2 && ok3 {
				t.Log("succeed")
			} else {
				t.Error("failed")
			}
		}
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
	res, err = client.Get(HTTPBIN + "/get")
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
