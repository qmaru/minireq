# minireq

Simple HTTP Client.

## Feature

+ Socks5 Proxy
+ HTTP Basic Auth
+ Params
+ JSON
+ Form
+ Upload File
+ Raw Response
+ JSON Response
+ Cookies

## Example

```go
client := NewClient()
params := Params{"foo": "bar"}
res, _ := client.Get("https://postman-echo.com/get", params)
data, _ := res.RawJSON()
fmt.Println(data)
```
