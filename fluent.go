package fluent

import (
	"fmt"
	msgpack "github.com/ugorji/go-msgpack"
	"net"
	"strconv"
	"time"
)

const (
	defaultHost = "127.0.0.1"
	defaultPort = 24224
)

type Config struct {
	FluentPort int
	FluentHost string
}

type Fluent struct {
	Config
	conn net.Conn
}

func New(config Config) (f *Fluent) {
	if config.FluentHost == "" {
		config.FluentHost = defaultHost
	}
	if config.FluentPort == 0 {
		config.FluentPort = defaultPort
	}
	f = &Fluent{
		Config: config,
	}
	return
}

func (f *Fluent) Connect() (err error) {
	f.conn, err = net.Dial("tcp", f.Config.FluentHost+":"+strconv.Itoa(f.Config.FluentPort))
	return
}

func (f *Fluent) Post(tag, message string) {
	timeNow := time.Now().Unix()
	msg := []interface{}{tag, timeNow, message}
	data, err := msgpack.Marshal(msg)
	if err != nil {
		fmt.Println(err)
	}
	f.Send(data)
}

func (f *Fluent) Send(data []byte) (err error) {
	_, err = f.conn.Write(data)
	return
}

func (f *Fluent) Close() (err error) {
	f.conn.Close()
	return
}
