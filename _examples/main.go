package main

import (
	"fmt"
	"github.com/t-k/fluent-logger-golang/fluent"
)

func main() {
	logger, err := fluent.New(fluent.Config{FluentPort: 24224, FluentHost: "127.0.0.1"})
	if err != nil {
		fmt.Println(err)
	}
	defer logger.Close()
	tag := "myapp.access"
	var data = map[string]string{
		"foo":  "bar",
		"hoge": "hoge"}
	logger.Post(tag, data)
}
