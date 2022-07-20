package minireq

import (
	"strings"

	"testing"
)

const HTTPBIN string = "https://postman-echo.com"

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
			jsonData := rawData.(map[string]interface{})
			args := jsonData["args"].(map[string]interface{})
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
	client.Socks5Address = "127.0.0.1:1080"
	res, err := client.Get(HTTPBIN + "/ip")
	if err != nil {
		t.Error(err)
	} else {
		rawData, err := res.RawJSON()
		if err != nil {
			t.Error(err)
		} else {
			jsonData := rawData.(map[string]interface{})
			ip := jsonData["ip"]
			t.Log(ip)
		}
	}
}

func TestAuth(t *testing.T) {
	client := NewClient()
	auth := Auth{"postman", "password"}
	res, err := client.Get(HTTPBIN+"/basic-auth", auth)
	if err != nil {
		t.Error(err)
	} else {
		rawData, err := res.RawJSON()
		if err != nil {
			t.Error(err)
		} else {
			jsonData := rawData.(map[string]interface{})
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
			jsonData := rawData.(map[string]interface{})
			form := jsonData["form"].(map[string]interface{})
			headers := jsonData["headers"].(map[string]interface{})
			contentType := headers["content-type"].(string)
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
			jsonData := rawData.(map[string]interface{})
			form := jsonData["form"].(map[string]interface{})
			files := jsonData["files"].(map[string]interface{})
			headers := jsonData["headers"].(map[string]interface{})
			contentType := headers["content-type"].(string)

			_, ok1 := form["foo"]
			_, ok2 := files["go.mod"]
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
			jsonData := rawData.(map[string]interface{})
			json := jsonData["json"].(map[string]interface{})
			headers := jsonData["headers"].(map[string]interface{})
			contentType := headers["content-type"].(string)
			if _, ok := json["foo"]; ok && contentType == "application/json" {
				t.Log("succeed")
			} else {
				t.Error("failed")
			}
		}
	}
}
