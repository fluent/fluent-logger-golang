package main

import (
	"fmt"
	"time"

	"../fluent"

	"github.com/Sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	logger, err := fluent.New(fluent.Config{FluentPort: 24224, FluentHost: "127.0.0.1"})
	if err != nil {
		fmt.Println(err)
	}
	defer logger.Close()
	tag := "myapp.access"
	var data = map[string]string{
		"foo":  "bar",
		"hoge": "hoge"}
	i := 0
	for i < 100 {
		e := logger.Post(tag, data)
		if e != nil {
			logrus.Debugf("Error while posting log: %s", e)
		} else {
			logrus.Debug("Success to post log")
		}
		i = i + 1
		time.Sleep(1000 * time.Millisecond)
	}
}
