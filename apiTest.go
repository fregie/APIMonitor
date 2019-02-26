package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"

	ypclnt "github.com/yunpian/yunpian-go-sdk/sdk"
)

var YP YunPianInfo
var mailInfo MailInfo

const configFile = "./config.json"

type testItem struct {
	Name           string            `json:"name"`
	URL            string            `json:"url"`
	HttpProto      string            `json:"httpProto"`
	RequestTimeout string            `json:"requestTimeout"`
	ServerList     map[string]string `jsong:"serverList"`
	Interval       string            `json:"interval"`
}

type YunPianInfo struct {
	Enable    bool     `json:"enable"`
	APIKey    string   `json:"apiKey"`
	Phones    []string `json:"phones"`
	SmsPrefix string   `json:"smsPrefix"`
}

type MailInfo struct {
	Enable   bool     `json:"enable"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	Smtp     string   `json:"smtp"`
	To       []string `json:"to"`
}

type config struct {
	MailInfo MailInfo    `json:"mail"`
	YPInfo   YunPianInfo `json:"YunPian"`
	TestItem []testItem  `json:"testItem"`
}

func main() {
	config := loadConfig(configFile)
	YP = config.YPInfo
	mailInfo = config.MailInfo

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
					fmt.Printf("[%s]: %s\n", name, err)
					if lastResult[name] == true {
						sendMSG(fmt.Sprintf("[%s](%s) 请求接口失败: %s", name, ip, err))
						sendEmail(fmt.Sprintf("[%s](%s) 请求接口失败: %s", name, ip, err))
						lastResult[name] = false
					}
				} else {
					fmt.Printf("[%s]: OK\n", name)
					if lastResult[name] == false {
						sendMSG(fmt.Sprintf("[%s](%s) 恢复正常", name, ip))
						sendEmail(fmt.Sprintf("[%s](%s) 恢复正常", name, ip))
						lastResult[name] = true
					}
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
	if resp.StatusCode >= 500 {
		return false, errors.New(fmt.Sprintf("status code(%d) not ok", resp.StatusCode))
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

func sendEmail(msg string) {
	if !mailInfo.Enable {
		return
	}
	auth := smtp.PlainAuth("", mailInfo.Username, mailInfo.Password, mailInfo.Smtp)
	to := mailInfo.To
	nickname := "服务器小管家"
	user := mailInfo.Username
	subject := "服务器异常告警"
	content_type := "Content-Type: text/plain; charset=UTF-8"
	body := msg
	context := []byte("To: " + strings.Join(to, ",") + "\r\nFrom: " + nickname +
		"<" + user + ">\r\nSubject: " + subject + "\r\n" + content_type + "\r\n\r\n" + body)
	err := smtp.SendMail("smtp.qq.com:25", auth, user, to, context)
	if err != nil {
		fmt.Printf("send mail error: %v", err)
	}
}

func sendMSG(msg string) {
	if !YP.Enable {
		return
	}
	// 发送短信
	clnt := ypclnt.New(YP.APIKey)
	param := ypclnt.NewParam(2)
	param[ypclnt.TEXT] = YP.SmsPrefix + msg
	for _, phone := range YP.Phones {
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
