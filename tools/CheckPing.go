package main

import (
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/bndr/gotabulate"
	pingunix "github.com/sparrc/go-ping"
	"gopkg.in/yaml.v1"
)

type config struct {
	ServerAddrs []string `yaml:"servers"`
}

func main() {
	conf, _ := loadConfig("./config.yaml")
	data := make([][]interface{}, len(conf.ServerAddrs))
	wg := sync.WaitGroup{}

	for i, s := range conf.ServerAddrs {
		wg.Add(1)
		go func(i int, s string) {
			pinger, err := pingunix.NewPinger(s)
			if err != nil {
				fmt.Printf("[%s]: %s", s, err)
				return
			}
			pinger.Count = 4
			pinger.Interval = 200 * time.Millisecond
			pinger.Timeout = 5 * time.Second
			pinger.Run()
			stats := pinger.Statistics()
			data[i] = []interface{}{s, int(stats.PacketLoss), stats.AvgRtt.Seconds() * 1000}
			wg.Done()
		}(i, s)
	}
	wg.Wait()

	t := gotabulate.Create(data)
	t.SetHeaders([]string{"IP", "Loss", "Delay"})
	t.SetEmptyString("NaN")
	t.SetAlign("right")
	fmt.Println(t.Render("grid"))
}

func loadConfig(fileName string) (*config, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	c := &config{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}
