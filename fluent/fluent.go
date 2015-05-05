package fluent

import (
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	defaultHost                   = "127.0.0.1"
	defaultPort                   = 24224
	defaultTimeout                = 3 * time.Second
	defaultBufferLimit            = 8 * 1024 * 1024
	defaultRetryWait              = 500
	defaultMaxRetry               = 13
	defaultReconnectWaitIncreRate = 1.5
)

type Config struct {
	FluentPort  int
	FluentHost  string
	Timeout     time.Duration
	BufferLimit int
	RetryWait   int
	MaxRetry    int
	TagPrefix   string
}

type Fluent struct {
	Config
	conn         net.Conn
	pending      []byte
	reconnecting bool
	mu           sync.Mutex
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
	if config.RetryWait == 0 {
		config.RetryWait = defaultRetryWait
	}
	if config.MaxRetry == 0 {
		config.MaxRetry = defaultMaxRetry
	}
	f = &Fluent{Config: config, reconnecting: false}
	err = f.connect()
	return
}

// Post writes the output for a logging event.
//
// Examples:
//
//  // send string
//  f.Post("tag_name", "data")
//
//  // send map[string]
//  mapStringData := map[string]string{
//  	"foo":  "bar",
//  }
//  f.Post("tag_name", mapStringData)
//
//  // send message with specified time
//  mapStringData := map[string]string{
//  	"foo":  "bar",
//  }
//  tm := time.Now()
//  f.PostWithTime("tag_name", tm, mapStringData)
//
//  // send struct
//  structData := struct {
//  		Name string `codec:"name"`
//  } {
//  		"john smith",
//  }
//  f.Post("tag_name", structData)
//
func (f *Fluent) Post(tag string, message interface{}) {
	timeNow := time.Now()
	f.PostWithTime(tag, timeNow, message)
}

func (f *Fluent) PostWithTime(tag string, tm time.Time, message interface{}) {
	if len(f.TagPrefix) > 0 {
		tag = f.TagPrefix + "." + tag
	}
	if data, dumperr := f.EncodeData(tag, tm, message); dumperr != nil {
		fmt.Println("fluent#Post: can't convert to msgpack:", message, dumperr)
	} else {
		f.PostRawData(data)
	}
}

func (f *Fluent) PostRawData(data []byte) {
	f.mu.Lock()
	f.pending = append(f.pending, data...)
	f.mu.Unlock()
	if err := f.send(); err != nil {
		f.close()
		if len(f.pending) > f.Config.BufferLimit {
			f.flushBuffer()
		}
	} else {
		f.flushBuffer()
	}
}

func (f *Fluent) EncodeData(tag string, tm time.Time, message interface{}) (data []byte, err error) {
	timeUnix := tm.Unix()
	msg := &Message{Tag: tag, Time: timeUnix, Record: message}
	data, err = msg.MarshalMsg(nil)
	return
}

// Close closes the connection.
func (f *Fluent) Close() (err error) {
	if len(f.pending) > 0 {
		_ = f.send()
	}
	err = f.close()
	return
}

// close closes the connection.
func (f *Fluent) close() (err error) {
	if f.conn != nil {
		f.mu.Lock()
		defer f.mu.Unlock()
	} else {
		return
	}
	if f.conn != nil {
		f.conn.Close()
		f.conn = nil
	}
	return
}

// connect establishes a new connection using the specified transport.
func (f *Fluent) connect() (err error) {
	f.conn, err = net.DialTimeout("tcp", f.Config.FluentHost+":"+strconv.Itoa(f.Config.FluentPort), f.Config.Timeout)
	return
}

func e(x, y float64) int {
	return int(math.Pow(x, y))
}

func (f *Fluent) reconnect() {
	go func() {
		for i := 0; ; i++ {
			err := f.connect()
			if err == nil {
				f.mu.Lock()
				f.reconnecting = false
				f.mu.Unlock()
				break
			} else {
				if i == f.Config.MaxRetry {
					panic("fluent#reconnect: failed to reconnect!")
				}
				waitTime := f.Config.RetryWait * e(defaultReconnectWaitIncreRate, float64(i-1))
				time.Sleep(time.Duration(waitTime) * time.Millisecond)
			}
		}
	}()
}

func (f *Fluent) flushBuffer() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pending = f.pending[0:0]
}

func (f *Fluent) send() (err error) {
	if f.conn == nil {
		if f.reconnecting == false {
			f.mu.Lock()
			f.reconnecting = true
			f.mu.Unlock()
			f.reconnect()
		}
		err = errors.New("fluent#send: can't send logs, client is reconnecting")
	} else {
		f.mu.Lock()
		_, err = f.conn.Write(f.pending)
		f.mu.Unlock()
	}
	return
}
