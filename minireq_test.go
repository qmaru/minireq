package minireq

import (
	"testing"
)

func TestGetRaw(t *testing.T) {
	request, err := Requests()
	if err != nil {
		t.Error(t.Name() + " Error")
	}
	res, err := request.Get("https://httpbin.org/get")
	if err != nil {
		t.Error(t.Name() + " Error")
	}
	_, err = res.RawJSON()
	if err != nil {
		t.Error(t.Name() + " Error")
	}
	t.Log(t.Name() + " Succeed")
}

func TestGetWithParam(t *testing.T) {
	request, err := Requests()
	if err != nil {
		t.Error(t.Name() + " Error")
	}
	headers := Headers{
		"User-Agent": "MyUserAgent",
	}
	params := Params{
		"key": "This is a get!",
	}
	res, err := request.Get("https://httpbin.org/get", headers, params)
	if err != nil {
		t.Error(t.Name() + " Error")
	}
	_, err = res.RawJSON()
	if err != nil {
		t.Error(t.Name() + " Error")
	}
	t.Log(t.Name() + " Succeed")
}

func TestPostRaw(t *testing.T) {
	request, err := Requests()
	if err != nil {
		t.Error(t.Name() + " Error")
	}
	res, err := request.Post("https://httpbin.org/post")
	if err != nil {
		t.Error(t.Name() + " Error")
	}
	_, err = res.RawJSON()
	if err != nil {
		t.Error(t.Name() + " Error")
	}
	t.Log(t.Name() + " Succeed")
}
