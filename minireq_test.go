package minireq

import (
	"testing"
)

func TestGetRaw(t *testing.T) {
	request := Requests()
	res := request.Get(
		"https://httpbin.org/get",
	)
	resJSON := res.RawJSON()
	if resJSON != nil {
		t.Log(t.Name() + " Succeed")
	} else {
		t.Error(t.Name() + " Error")
	}
}

func TestGetWithParam(t *testing.T) {
	request := Requests()
	headers := Headers{
		"User-Agent": "MyUserAgent",
	}
	params := Params{
		"key": "This is a get!",
	}
	res := request.Get(
		"https://httpbin.org/get",
		headers,
		params,
	)
	resJSON := res.RawJSON()
	if resJSON != nil {
		t.Log(t.Name() + " Succeed")
	} else {
		t.Error(t.Name() + " Error")
	}
}

func TestPostRaw(t *testing.T) {
	request := Requests()
	res := request.Post(
		"https://httpbin.org/post",
	)
	resJSON := res.RawJSON()
	if resJSON != nil {
		t.Log(t.Name() + " Succeed")
	} else {
		t.Error(t.Name() + " Error")
	}
}
