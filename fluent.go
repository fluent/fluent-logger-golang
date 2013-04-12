package fluent

import (
	"container/list"
	"fmt"
	msgpack "github.com/ugorji/go-msgpack"
	"net"
	"strconv"
	"time"
)

const (
	defaultHost        = "127.0.0.1"
	defaultPort        = 24224
	defaultTimeout     = 3 * time.Second
	defaultBufferLimit = 8 * 1024 * 1024
)

type Config struct {
	FluentPort  int
	FluentHost  string
	Timeout     time.Duration
	BufferLimit int
}

type Fluent struct {
	Config
	conn    net.Conn
	pending list.List
}

// New creates a new Logger.
func New(config Config) (f *Fluent, err error) {
	if config.FluentHost == "" {
		config.FluentHost = defaultHost
	}
	if config.FluentPort == 0 {
		config.FluentPort = defaultPort
	}
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}
	if config.BufferLimit == 0 {
		config.BufferLimit = defaultBufferLimit
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
	data, dumperr := msgpack.Marshal(msg)
	if dumperr != nil {
		fmt.Println("Fluent: Can't convert to msgpack:", msg, dumperr)
	} else {
		err := f.send(data)
		if err != nil && len(data) <= f.Config.BufferLimit {
			f.pending.PushBack(data)
			f.Close()
		} else {
			f.pending.Init()
		}
	}
}

// Close closes the connection.
func (f *Fluent) Close() (err error) {
	if f.conn != nil {
		f.conn.Close()
		f.conn = nil
	}
	return
}

// Connect establishes a new connection using the specified transport.
func (f *Fluent) connect() (err error) {
	f.conn, err = net.DialTimeout("tcp", f.Config.FluentHost+":"+strconv.Itoa(f.Config.FluentPort), f.Config.Timeout)
	return
}

func (f *Fluent) send(data []byte) (err error) {
	if f.conn == nil {
		err = f.connect()
	}
	if f.pending.Len() > 0 {
		for e := f.pending.Front(); e != nil; e = e.Next() {
			_, err = f.conn.Write(e.Value.([]byte))
		}
	} else {
		_, err = f.conn.Write(data)
	}
	return
}
