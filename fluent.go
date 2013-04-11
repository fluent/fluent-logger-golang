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

// New creates a new Logger.
func New(config Config) (f *Fluent, err error) {
	if config.FluentHost == "" {
		config.FluentHost = defaultHost
	}
	if config.FluentPort == 0 {
		config.FluentPort = defaultPort
	}
	f = &Fluent{
		Config: config,
	}
	err = f.connect()
	return
}

// Post writes the output for a logging event.
func (f *Fluent) Post(tag string, message interface{}) {
	timeNow := time.Now().Unix()
	msg := []interface{}{tag, timeNow, message}
	data, err := msgpack.Marshal(msg)
	if err != nil {
		fmt.Println(err)
	}
	f.send(data)
}

// Close closes the connection.
func (f *Fluent) Close() (err error) {
	f.conn.Close()
	return
}

// Connect establishes a new connection using the specified transport.
func (f *Fluent) connect() (err error) {
	f.conn, err = net.Dial("tcp", f.Config.FluentHost+":"+strconv.Itoa(f.Config.FluentPort))
	return
}

func (f *Fluent) send(data []byte) (err error) {
	if f.conn == nil {
		f.connect()
	}
	_, err = f.conn.Write(data)
	return
}
