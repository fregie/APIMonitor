package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v1"

	ypclnt "github.com/yunpian/yunpian-go-sdk/sdk"
)

var YP YunPianInfo
var mailInfo MailInfo

var (
	configFile = flag.String("c", "./config.yaml", "config file path")
)

type testItem struct {
	Name           string            `yaml:"name"`
	URL            string            `yaml:"url"`
	HttpProto      string            `yaml:"httpProto"`
	RequestTimeout string            `yaml:"requestTimeout"`
	ServerList     map[string]string `yaml:"serverList"`
	Interval       string            `yaml:"interval"`
	AlertTimes     uint              `yaml:"alertTimes"`
	CertVerify     bool              `yaml:"crtVerify"`
	Cert           string            `yaml:"cert"`
	Key            string            `yaml:"key"`
}

type YunPianInfo struct {
	Enable    bool     `yaml:"enable"`
	APIKey    string   `yaml:"apiKey"`
	Phones    []string `yaml:"phones"`
	SmsPrefix string   `yaml:"smsPrefix"`
}

type MailInfo struct {
	Enable   bool     `yaml:"enable"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
	Smtp     string   `yaml:"smtp"`
	To       []string `yaml:"to"`
}

type config struct {
	MailInfo MailInfo    `yaml:"mail"`
	YPInfo   YunPianInfo `yaml:"YunPian"`
	TestItem []testItem  `yaml:"testItem"`
}

func main() {
	flag.Parse()
	config := loadConfig(*configFile)
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
	log.Printf("Start testing [%s]", item.Name)
	lastResult := map[string]uint{}
	rMutex := sync.Mutex{}

	for {
		wg := sync.WaitGroup{}
		for ip, name := range item.ServerList {
			wg.Add(1)
			go func(ip, name string) {
				// log.Printf("testing %s", ip)
				ok, err := testServer(ip, item.HttpProto, item.URL, item.RequestTimeout, item.Cert, item.Key, item.CertVerify)
				if _, ok := lastResult[ip]; !ok {
					rMutex.Lock()
					lastResult[ip] = 0
					rMutex.Unlock()
				}
				if !ok {
					rMutex.Lock()
					lastResult[ip]++
					rMutex.Unlock()
					log.Printf("[%s]: %s", name, err)
					if lastResult[ip] == item.AlertTimes {
						log.Printf("send")
						sendMSG(fmt.Sprintf("[%s](%s) 请求接口失败: %s", name, ip, err))
						sendEmail(fmt.Sprintf("[%s](%s) 请求接口失败: %s", name, ip, err))
					}
				} else {
					log.Printf("[%s(%s)]: OK", name, ip)
					if lastResult[ip] >= item.AlertTimes {
						sendMSG(fmt.Sprintf("[%s](%s) 恢复正常", name, ip))
						sendEmail(fmt.Sprintf("[%s](%s) 恢复正常", name, ip))
						rMutex.Lock()
						lastResult[ip] = 0
						rMutex.Unlock()
					}
				}
				wg.Done()
			}(ip, name)
		}
		wg.Wait()
		t, _ := time.ParseDuration(item.Interval)
		time.Sleep(t)
	}
}

func testServer(serverIP, proto, url, requestTimeout, cert, key string, certVerify bool) (bool, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !certVerify,
		},
	}
	if cert != "" && key != "" {
		cert, err := tls.X509KeyPair([]byte(cert), []byte(key))
		if err != nil {
			panic("init cert failed")
		}
		certs := []tls.Certificate{cert}
		tr.TLSClientConfig.Certificates = certs
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
	if resp.StatusCode >= 400 {
		return false, errors.New(fmt.Sprintf("status code(%d) not ok", resp.StatusCode))
	}

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
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil
	}
	c := &config{}
	err = yaml.Unmarshal(data, c)
	if err != nil {
		return nil
	}

	return c
}
