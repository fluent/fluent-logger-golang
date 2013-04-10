package main

import (
  "github.com/t-k/fluent-logger-golang"
  "fmt"
)

func main() {
  logger := fluent.New(fluent.Config{})
  err := logger.Connect()
  if err != nil {
    fmt.Println(err)
  }
  defer logger.Close()
  tag := "myapp.access"
  message := "testing"
  logger.Post(tag, message)
}