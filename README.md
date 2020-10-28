# minireq

简单的 HTTP 封装，仅包括 Get / Post

## Example
### 1. GET
```go
request := minireq.Requests()
res := request.Get(
    "https://httpbin.org/get",
)
fmt.Println(res.RawJSON())
```
### 2. Get With Param
```go
request := minireq.Requests()
headers := minireq.Headers{
    "User-Agent": "MyUserAgent",
}
params := minireq.Params{
    "foo": "This is a get!",
}
res := request.Get(
    "https://httpbin.org/get",
    headers,
    params,
)
fmt.Println(res.RawJSON())
```
### 3. Get With Auth
```go
request := minireq.Requests()
authData := minireq.Auth{
    "admin",
    "123456",
}
res := request.Get(
    "http://example.com/auth",
    authData,
)
fmt.Println(string(res.RawData()))
```
### 4. POST
```go
request := minireq.Requests()
res := request.Post(
    "https://httpbin.org/post",
)
fmt.Println(res.RawJSON())
```
### 5. Post By JSON
```go
request := minireq.Requests()
data := minireq.JSONData{
    "username": "admin",
    "password": "123456",
}
res := request.Post(
    "https://httpbin.org/json",
    data,
)
fmt.Println(string(res.RawData()))
```
### 6. Post By Form
```go
request := minireq.Requests()
data := minireq.FormData{
    "username": "admin",
    "password": "123456",
}
res := request.Post(
    "https://httpbin.org/json",
    data,
)
fmt.Println(string(res.RawData()))
```
### 7. Post Files
```go
request := minireq.Requests()
fdata := minireq.FileData{
    "files[]": []string{
        "go.mod",
        "go.sum",
    },
}
request.Post(
    "http://example.com/auth",
    fdata,
)
```
### 8. Set Proxy
```go
request := minireq.Requests()
request.Proxy("127.0.0.1:1080")
res := request.Get(
    "https://httpbin.org/get",
)
fmt.Println(string(res.RawData()))
```
## 参考
[requests](https://github.com/asmcos/requests)