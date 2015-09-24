package fluent

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"strconv"
	"time"

	"golang.org/x/net/context"
)

const (
	defaultHost                   = "127.0.0.1"
	defaultNetwork                = "tcp"
	defaultSocketPath             = ""
	defaultPort                   = 24224
	defaultTimeout                = 3 * time.Second
	defaultBufferLimit            = 8 * 1024 * 1024
	defaultRetryWait              = 500
	defaultMaxRetry               = 13
	defaultReconnectWaitIncreRate = 1.5
	defaultSyncPost               = false
)

type Config struct {
	FluentPort       int
	FluentHost       string
	FluentNetwork    string
	FluentSocketPath string
	Timeout          time.Duration
	BufferLimit      int
	RetryWait        int
	MaxRetry         int
	TagPrefix        string
	SyncPost         bool
}

type Fluent struct {
	Config
	conn   io.WriteCloser
	buf    []byte
	postCh chan post
	result chan error
	ctx    context.Context
	cancel context.CancelFunc
}

type post struct {
	result chan error
	data   []byte
}

// New creates a new Logger.
func New(config Config) (f *Fluent, err error) {
	if config.FluentNetwork == "" {
		config.FluentNetwork = defaultNetwork
	}
	if config.FluentHost == "" {
		config.FluentHost = defaultHost
	}
	if config.FluentPort == 0 {
		config.FluentPort = defaultPort
	}
	if config.FluentSocketPath == "" {
		config.FluentSocketPath = defaultSocketPath
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
	if config.SyncPost == false {
		config.SyncPost = defaultSyncPost
	}
	f = &Fluent{
		Config: config,
		postCh: make(chan post),
		result: make(chan error),
	}

	f.ctx, f.cancel = context.WithCancel(context.Background())

	if err = f.connect(); err != nil {
		return
	}
	if f.SyncPost {
		go f.syncPostWorker(f.ctx)
	} else {
		go f.spoolWorker(f.ctx)
	}

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
//  		Name string `msg:"name"`
//  } {
//  		"john smith",
//  }
//  f.Post("tag_name", structData)
//
func (f *Fluent) Post(tag string, message interface{}) error {
	timeNow := time.Now()
	return f.PostWithTime(tag, timeNow, message)
}

func (f *Fluent) PostWithTime(tag string, tm time.Time, message interface{}) error {
	if len(f.TagPrefix) > 0 {
		tag = f.TagPrefix + "." + tag
	}

	msg := reflect.ValueOf(message)
	msgtype := msg.Type()

	if msgtype.Kind() == reflect.Struct {
		// message should be tagged by "codec" or "msg"
		kv := make(map[string]interface{})
		fields := msgtype.NumField()
		for i := 0; i < fields; i++ {
			field := msgtype.Field(i)
			name := field.Name
			if n1 := field.Tag.Get("msg"); n1 != "" {
				name = n1
			} else if n2 := field.Tag.Get("codec"); n2 != "" {
				name = n2
			}
			kv[name] = msg.FieldByIndex(field.Index).Interface()
		}
		return f.EncodeAndPostData(tag, tm, kv)
	}

	if msgtype.Kind() != reflect.Map {
		return errors.New("messge must be a map")
	} else if msgtype.Key().Kind() != reflect.String {
		return errors.New("map keys must be strings")
	}

	kv := make(map[string]interface{})
	for _, k := range msg.MapKeys() {
		kv[k.String()] = msg.MapIndex(k).Interface()
	}

	return f.EncodeAndPostData(tag, tm, kv)
}

func (f *Fluent) EncodeAndPostData(tag string, tm time.Time, message interface{}) error {
	data, dumperr := f.EncodeData(tag, tm, message)
	if dumperr != nil {
		return fmt.Errorf("fluent#EncodeAndPostData: can't convert '%s' to msgpack:%s", message, dumperr)
		// fmt.Println("fluent#Post: can't convert to msgpack:", message, dumperr)
	}
	return f.PostRawDataWithResult(data)
}

func (f *Fluent) PostRawData(data []byte) {
	if err := f.PostRawDataWithResult(data); err != nil {
		panic(err)
	}
}

func (f *Fluent) PostRawDataWithResult(data []byte) (err error) {
	buf := make([]byte, len(data))
	copy(buf, data)
	postResult := make(chan error)
	select {
	case f.postCh <- post{result: postResult, data: buf}:
		err := <-postResult
		return err
	case err = <-f.result:
		f.close()
	}
	return err
}

func (f *Fluent) EncodeData(tag string, tm time.Time, message interface{}) (data []byte, err error) {
	timeUnix := tm.Unix()
	msg := &Message{Tag: tag, Time: timeUnix, Record: message}
	data, err = msg.MarshalMsg(nil)
	return
}

// Close closes the connection.
func (f *Fluent) Close() (err error) {
	f.cancel()
	for err = range f.result {
		f.close()
		break
	}
	return
}

// close closes the connection.
func (f *Fluent) close() (err error) {
	if f.conn == nil {
		return
	}
	f.conn.Close()
	f.conn = nil
	return
}

// connect establishes a new connection using the specified transport.
func (f *Fluent) connect() (err error) {
	switch f.Config.FluentNetwork {
	case "tcp":
		f.conn, err = net.DialTimeout(f.Config.FluentNetwork, f.Config.FluentHost+":"+strconv.Itoa(f.Config.FluentPort), f.Config.Timeout)
	case "unix":
		f.conn, err = net.DialTimeout(f.Config.FluentNetwork, f.Config.FluentSocketPath, f.Config.Timeout)
	default:
		err = net.UnknownNetworkError(f.Config.FluentNetwork)
	}
	return
}

func e(x, y float64) int {
	return int(math.Pow(x, y))
}

func (f *Fluent) reconnect() error {
	for i := 0; ; i++ {
		err := f.connect()
		if err == nil {
			return nil
		}
		if i == f.Config.MaxRetry {
			return errors.New("fluent#reconnect: failed to reconnect!")
		}
		waitTime := f.Config.RetryWait * e(defaultReconnectWaitIncreRate, float64(i-1))
		time.Sleep(time.Duration(waitTime) * time.Millisecond)
	}
}

func (f *Fluent) flushBuffer() {
	f.buf = f.buf[0:0]
}

func (f *Fluent) send(data []byte) (err error) {
	for {
		if f.conn == nil {
			if err = f.reconnect(); err != nil {
				return
			}
		}
		if _, err = f.conn.Write(data); err == nil {
			return
		}
		f.close()
	}
}

func (f *Fluent) spoolWorker(ctx context.Context) {
	senderResult := make(chan error)
	sendChCh := f.sendWorker(ctx, senderResult)
	for {
		var receive chan chan []byte
		if len(f.buf) > 0 {
			receive = sendChCh
		}
		select {
		case p := <-f.postCh:
			if len(f.buf) > f.Config.BufferLimit {
				p.result <- fmt.Errorf("fluent#spooler: Post has failed, buffer size exceeds the limit. [ buffer size: %d > BufferLimit: %d ]", len(f.buf), f.Config.BufferLimit)
				break
			}
			f.buf = append(f.buf, p.data...)
			p.result <- nil
		case sendCh := <-receive:
			buf := make([]byte, len(f.buf))
			copy(buf, f.buf)
			f.flushBuffer()
			sendCh <- buf
		case <-ctx.Done():
			<-senderResult
			err := f.send(f.buf)
			f.result <- err
			close(f.result)
			return
		case err := <-senderResult:
			f.result <- err
			close(f.result)
			return
		}
	}
}

func (f *Fluent) syncPostWorker(ctx context.Context) {
	for {
		select {
		case p := <-f.postCh:
			err := f.send(p.data)
			p.result <- err
		case <-ctx.Done():
			f.result <- ctx.Err()
			return
		}
	}
}

func (f *Fluent) sendWorker(ctx context.Context, result chan error) chan chan []byte {
	sendCh := make(chan chan []byte)
	go func() {
		bufCh := make(chan []byte)
		for {
			select {
			case sendCh <- bufCh:
			case <-ctx.Done():
				result <- ctx.Err()
				return
			}
			select {
			case data := <-bufCh:
				if err := f.send(data); err != nil {
					result <- err
					return
				}
			case <-ctx.Done():
				result <- ctx.Err()
				return
			}
		}
	}()
	return sendCh
}
