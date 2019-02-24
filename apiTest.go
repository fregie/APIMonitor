package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	ypclnt "github.com/yunpian/yunpian-go-sdk/sdk"
)

var APIKey string
var phones []string
var smsPrefix string

const configFile = "./config.json"

type testItem struct {
	Name           string            `json:"name"`
	URL            string            `json:"url"`
	HttpProto      string            `json:"httpProto"`
	RequestTimeout string            `json:"requestTimeout"`
	ServerList     map[string]string `jsong:"serverList"`
	Interval       string            `json:"interval"`
}

type config struct {
	APIKey    string     `json:"apiKey"`
	Phones    []string   `json:"phones"`
	SmsPrefix string     `json:"smsPrefix"`
	TestItem  []testItem `json:"testItem"`
}

func main() {
	config := loadConfig(configFile)
	APIKey = config.APIKey
	phones = config.Phones
	smsPrefix = config.SmsPrefix

	for _, item := range config.TestItem {
		go itemTest(item)
	}

	//block main routin
	c := make(chan int)
	<-c
}

func itemTest(item testItem) {
	lastResult := map[string]bool{}

	sig := make(chan int, 24)
	num := 0
	for {
		for ip, name := range item.ServerList {
			num++
			go func(ip, name string) {
				ok, err := testServer(ip, item.HttpProto, item.URL, item.RequestTimeout)
				if _, ok := lastResult[name]; !ok {
					lastResult[name] = true
				}
				if !ok {
					if lastResult[name] == true {
						sendMSG(fmt.Sprintf("[%s](%s) 请求接口失败: %s", name, ip, err))
						lastResult[name] = false
					}
					fmt.Printf("[%s]: %s\n", name, err)
				} else {
					if lastResult[name] == false {
						sendMSG(fmt.Sprintf("[%s](%s) 恢复正常", name, ip))
						lastResult[name] = true
					}
					fmt.Printf("[%s]: OK\n", name)
				}
				sig <- 1
			}(ip, name)
		}
		for msg := range sig {
			if msg == 1 {
				num--
				if num <= 0 {
					break
				}
			}
		}
		t, _ := time.ParseDuration(item.Interval)
		time.Sleep(t)
	}
}

func testServer(serverIP, proto, url, requestTimeout string) (bool, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	timeout, _ := time.ParseDuration(requestTimeout)
	client := &http.Client{Transport: tr, Timeout: timeout}
	resp, err := client.Get(fmt.Sprintf("%s://%s%s", proto, serverIP, url))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, errors.New("status code not ok")
	}

	// var f interface{}
	// err2 := json.Unmarshal(body, &f)
	// if err2 != nil {
	// 	return false, err
	// }
	// rspMap := f.(map[string]interface{})
	// if status, ok := rspMap["status"]; !ok || status.(string) == "fail" {
	// 	return false, errors.New(rspMap["error"].(string))
	// }

	// fmt.Print(string(body))
	return true, nil
}

func sendMSG(msg string) {
	// 发送短信
	clnt := ypclnt.New(APIKey)
	param := ypclnt.NewParam(2)
	param[ypclnt.TEXT] = smsPrefix + msg
	for _, phone := range phones {
		param[ypclnt.MOBILE] = phone
		clnt.Sms().SingleSend(param)
	}
}

func loadConfig(fileName string) *config {
	file, _ := os.Open(fileName)
	defer file.Close()
	decoder := json.NewDecoder(file)
	conf := new(config)
	err := decoder.Decode(&conf)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return conf
}
