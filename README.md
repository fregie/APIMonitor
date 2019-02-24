# APIMonitor
Check if the API is working regularly. Writen in golang

### Custom your config
```json
{
  "apiKey": "your YunPian apikay",
  "phones":["phone1", "phone2"],
  "smsPrefix": "your yun pian sms prefix config",  
  "testItem":[
    {
      "name":"api-test",
      "url":"/api/to/test",
      "httpProto": "https",
      "requestTimeout": "20s",
      "serverList":{
        "0.0.0.0": "server-1",
        "baidu.com": "domain-2"
      },
      "interval": "20s"
    }
  ]
}
```
### Test 
`go run apiTest.go`

### Build
`go build -o API-monitor apiTest.go`