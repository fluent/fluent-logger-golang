fluent-logger-golang
====

[![Build Status](https://travis-ci.org/fluent/fluent-logger-golang.png?branch=master)](https://travis-ci.org/fluent/fluent-logger-golang)

## A structured event logger for Fluentd (Golang)

## How to install

```
go get github.com/fluent/fluent-logger-golang/fluent
```

## Usage

Install the package with `go get` and use `import` to include it in your project.

```
import "github.com/fluent/fluent-logger-golang/fluent"
```

GoDoc: http://godoc.org/github.com/fluent/fluent-logger-golang/fluent

##Example

```go
package main

import (
  "github.com/fluent/fluent-logger-golang/fluent"
  "fmt"
  "time"
)

func main() {
  logger, err := fluent.New(fluent.Config{})
  if err != nil {
    fmt.Println(err)
  }
  defer logger.Close()
  tag := "myapp.access"
  var data = map[string]string{
    "foo":  "bar",
    "hoge": "hoge",
  }
  error := logger.Post(tag, data)
  // error := logger.Post(tag, time.Time.Now(), data)
  if error != nil {
    panic(error)
  }
}
```

`data` must be a value like `map[string]literal`, `map[string]interface{}` or `struct`. Logger refers tags `msg` or `codec` of each fields of structs.

## Setting config values

```go
f := fluent.New(fluent.Config{FluentPort: 80, FluentHost: "example.com"})
```

## Buffer overflow

You can inject your own custom func to handle buffer overflow in the event of connection failure.
This will mitigate the loss of data instead of simply rejecting data and ending up throwing data away.

Your func must accept a single argument, which will be the internal buffer of messages from the logger.
This func is also called when logger.Close() failed to send the remaining internal buffer of messages.
A typical use-case for this would be writing to disk or possibly writing to Redis.

## Tests
```
go test
```
