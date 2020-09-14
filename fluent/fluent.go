package fluent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"bytes"
	"encoding/base64"
	"encoding/binary"
	"math/rand"

	"github.com/tinylib/msgp/msgp"
)

const (
	defaultHost                   = "127.0.0.1"
	defaultNetwork                = "tcp"
	defaultSocketPath             = ""
	defaultPort                   = 24224
	defaultTimeout                = 3 * time.Second
	defaultWriteTimeout           = time.Duration(0) // Write() will not time out
	defaultBufferLimit            = 8 * 1024
	defaultRetryWait              = 500
	defaultMaxRetryWait           = 60000
	defaultMaxRetry               = 13
	defaultReconnectWaitIncreRate = 1.5
	// Default sub-second precision value to false since it is only compatible
	// with fluentd versions v0.14 and above.
	defaultSubSecondPrecision = false
)

type Config struct {
	FluentPort         int           `json:"fluent_port"`
	FluentHost         string        `json:"fluent_host"`
	FluentNetwork      string        `json:"fluent_network"`
	FluentSocketPath   string        `json:"fluent_socket_path"`
	Timeout            time.Duration `json:"timeout"`
	WriteTimeout       time.Duration `json:"write_timeout"`
	BufferLimit        int           `json:"buffer_limit"`
	RetryWait          int           `json:"retry_wait"`
	MaxRetry           int           `json:"max_retry"`
	MaxRetryWait       int           `json:"max_retry_wait"`
	TagPrefix          string        `json:"tag_prefix"`
	Async              bool          `json:"async"`
	ForceStopAsyncSend bool          `json:"force_stop_async_send"`
	// Deprecated: Use Async instead
	AsyncConnect  bool `json:"async_connect"`
	MarshalAsJSON bool `json:"marshal_as_json"`

	// Sub-second precision timestamps are only possible for those using fluentd
	// v0.14+ and serializing their messages with msgpack.
	SubSecondPrecision bool `json:"sub_second_precision"`

	// RequestAck sends the chunk option with a unique ID. The server will
	// respond with an acknowledgement. This option improves the reliability
	// of the message transmission.
	RequestAck bool `json:"request_ack"`
}

type ErrUnknownNetwork struct {
	network string
}

func (e *ErrUnknownNetwork) Error() string {
	return "unknown network " + e.network
}

func NewErrUnknownNetwork(network string) error {
	return &ErrUnknownNetwork{network}
}

type msgToSend struct {
	ctx  context.Context
	data []byte
	ack  string
}

type bufferInput struct {
	msg    *msgToSend
	result chan<- error
}

type Fluent struct {
	Config

	dialer      dialer
	stopRunning chan bool
	pending     chan bufferInput
	wg          sync.WaitGroup
	resultPool  sync.Pool

	muconn sync.Mutex
	conn   net.Conn
}

// New creates a new Logger.
func New(config Config) (*Fluent, error) {
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}
	return newWithDialer(config, &net.Dialer{
		Timeout: config.Timeout,
	})
}

type dialer interface {
	Dial(string, string) (net.Conn, error)
}

type dialerWithContext interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

func newWithDialer(config Config, d dialer) (f *Fluent, err error) {
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
	if config.WriteTimeout == 0 {
		config.WriteTimeout = defaultWriteTimeout
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
	if config.MaxRetryWait == 0 {
		config.MaxRetryWait = defaultMaxRetryWait
	}
	if config.AsyncConnect {
		fmt.Fprintf(os.Stderr, "fluent#New: AsyncConnect is now deprecated, please use Async instead")
		config.Async = config.Async || config.AsyncConnect
	}
	f = &Fluent{
		Config:  config,
		dialer:  d,
		pending: make(chan bufferInput, config.BufferLimit),
		resultPool: sync.Pool{
			New: func() interface{} {
				return make(chan error, 1)
			},
		},
	}
	if !config.Async {
		if err = f.connect(context.Background()); err != nil {
			return
		}
	}

	f.wg.Add(1)
	go f.run()
	return
}

// Post writes the output for a logging event.
//
// Examples:
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
	return f.PostWithContext(context.Background(), tag, message)
}

func (f *Fluent) PostWithContext(ctx context.Context, tag string, message interface{}) error {
	timeNow := time.Now()
	return f.PostWithTimeAndContext(ctx, tag, timeNow, message)
}

func (f *Fluent) PostWithTime(tag string, tm time.Time, message interface{}) error {
	return f.PostWithTimeAndContext(context.Background(), tag, tm, message)
}

func (f *Fluent) PostWithTimeAndContext(ctx context.Context, tag string, tm time.Time, message interface{}) error {
	if len(f.TagPrefix) > 0 {
		tag = f.TagPrefix + "." + tag
	}

	if m, ok := message.(msgp.Marshaler); ok {
		return f.EncodeAndPostDataWithContext(ctx, tag, tm, m)
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
		return f.EncodeAndPostDataWithContext(ctx, tag, tm, kv)
	}

	if msgtype.Kind() != reflect.Map {
		return errors.New("fluent#PostWithTime: message must be a map")
	} else if msgtype.Key().Kind() != reflect.String {
		return errors.New("fluent#PostWithTime: map keys must be strings")
	}

	kv := make(map[string]interface{})
	for _, k := range msg.MapKeys() {
		kv[k.String()] = msg.MapIndex(k).Interface()
	}

	return f.EncodeAndPostDataWithContext(ctx, tag, tm, kv)
}

func (f *Fluent) EncodeAndPostData(tag string, tm time.Time, message interface{}) error {
	return f.EncodeAndPostDataWithContext(context.Background(), tag, tm, message)
}

func (f *Fluent) EncodeAndPostDataWithContext(ctx context.Context, tag string, tm time.Time, message interface{}) error {
	var msg *msgToSend
	var err error
	if msg, err = f.EncodeDataWithContext(ctx, tag, tm, message); err != nil {
		return fmt.Errorf("fluent#EncodeAndPostData: can't convert '%#v' to msgpack:%v", message, err)
	}
	return f.postRawData(msg)
}

// Deprecated: Use EncodeAndPostData instead
func (f *Fluent) PostRawData(msg *msgToSend) {
	f.postRawData(msg)
}

func (f *Fluent) postRawData(msg *msgToSend) error {
	if f.Config.Async {
		return f.appendBuffer(msg)
	}
	// Synchronous write
	return f.appendBufferBlocking(msg)
}

// For sending forward protocol adopted JSON
type MessageChunk struct {
	message Message
}

// Golang default marshaler does not support
// ["value", "value2", {"key":"value"}] style marshaling.
// So, it should write JSON marshaler by hand.
func (chunk *MessageChunk) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(chunk.message.Record)
	if err != nil {
		return nil, err
	}
	option, err := json.Marshal(chunk.message.Option)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("[\"%s\",%d,%s,%s]", chunk.message.Tag,
		chunk.message.Time, data, option)), err
}

// getUniqueID returns a base64 encoded unique ID that can be used for chunk/ack
// mechanism, see
// https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1#option
func getUniqueID(timeUnix int64) (string, error) {
	buf := bytes.NewBuffer(nil)
	enc := base64.NewEncoder(base64.StdEncoding, buf)
	if err := binary.Write(enc, binary.LittleEndian, timeUnix); err != nil {
		enc.Close()
		return "", err
	}
	if err := binary.Write(enc, binary.LittleEndian, rand.Uint64()); err != nil {
		enc.Close()
		return "", err
	}
	// encoder needs to be closed before buf.String(), defer does not work
	// here
	enc.Close()
	return buf.String(), nil
}

func (f *Fluent) EncodeData(tag string, tm time.Time, message interface{}) (msg *msgToSend, err error) {
	return f.EncodeDataWithContext(context.Background(), tag, tm, message)
}

func (f *Fluent) EncodeDataWithContext(ctx context.Context, tag string, tm time.Time, message interface{}) (msg *msgToSend, err error) {
	option := make(map[string]string)
	msg = &msgToSend{ctx: ctx}
	timeUnix := tm.Unix()
	if f.Config.RequestAck {
		var err error
		msg.ack, err = getUniqueID(timeUnix)
		if err != nil {
			return nil, err
		}
		option["chunk"] = msg.ack
	}
	if f.Config.MarshalAsJSON {
		m := Message{Tag: tag, Time: timeUnix, Record: message, Option: option}
		chunk := &MessageChunk{message: m}
		msg.data, err = json.Marshal(chunk)
	} else if f.Config.SubSecondPrecision {
		m := &MessageExt{Tag: tag, Time: EventTime(tm), Record: message, Option: option}
		msg.data, err = m.MarshalMsg(nil)
	} else {
		m := &Message{Tag: tag, Time: timeUnix, Record: message, Option: option}
		msg.data, err = m.MarshalMsg(nil)
	}
	return
}

// Close closes the connection, waiting for pending logs to be sent
func (f *Fluent) Close() (err error) {
	if f.Config.Async {
		if f.Config.ForceStopAsyncSend {
			f.stopRunning <- true
			close(f.stopRunning)
		}
		close(f.pending)
		f.wg.Wait()
	}
	f.close(f.conn)
	return
}

// appendBuffer appends data to buffer with lock.
func (f *Fluent) appendBuffer(msg *msgToSend) error {
	select {
	case f.pending <- bufferInput{msg: msg}:
	default:
		return fmt.Errorf("fluent#appendBuffer: Buffer full, limit %v", f.Config.BufferLimit)
	}
	return nil
}

// appendBufferWithFeedback appends data to buffer and waits for the result
func (f *Fluent) appendBufferBlocking(msg *msgToSend) error {
	result := f.resultPool.Get().(chan error)
	// write the data to the buffer and block if the buffer is full
	select {
	case f.pending <- bufferInput{msg: msg, result: result}:
		// don't do anything
	case <-msg.ctx.Done():
		// because the result channel is not used, it can safely be returned to the sync pool.
		f.resultPool.Put(result)
		return msg.ctx.Err()
	}

	select {
	case err := <-result:
		f.resultPool.Put(result)
		return err
	case <-msg.ctx.Done():
		// the context deadline has exceeded, but there is no result yet. So the result channel cannot be returned to
		// the pool, as it might be written later.
		return msg.ctx.Err()
	}
}

// close closes the connection.
func (f *Fluent) close(c net.Conn) {
	f.muconn.Lock()
	if f.conn != nil && f.conn == c {
		f.conn.Close()
		f.conn = nil
	}
	f.muconn.Unlock()
}

// connect establishes a new connection using the specified transport.
func (f *Fluent) connect(ctx context.Context) (err error) {
	var address string
	switch f.Config.FluentNetwork {
	case "tcp":
		address = f.Config.FluentHost + ":" + strconv.Itoa(f.Config.FluentPort)
	case "unix":
		address = f.Config.FluentSocketPath
	default:
		err = NewErrUnknownNetwork(f.Config.FluentNetwork)
		return
	}
	if d, ok := f.dialer.(dialerWithContext); ok {
		f.conn, err = d.DialContext(ctx, f.Config.FluentNetwork, address)
	} else {
		f.conn, err = f.dialer.Dial(f.Config.FluentNetwork, address)
	}

	return err
}

func (f *Fluent) run() {
	drainEvents := false
	var emitEventDrainMsg sync.Once
	for {
		select {
		case entry, ok := <-f.pending:
			if !ok {
				f.wg.Done()
				return
			}
			if drainEvents {
				emitEventDrainMsg.Do(func() { fmt.Fprintf(os.Stderr, "[%s] Discarding queued events...\n", time.Now().Format(time.RFC3339)) })
				continue
			}
			err := f.write(entry.msg)
			if entry.result != nil {
				entry.result <- err
				continue
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s] Unable to send logs to fluentd, reconnecting...\n", time.Now().Format(time.RFC3339))
			}
		}
		select {
		case stopRunning, ok := <-f.stopRunning:
			if stopRunning || !ok {
				drainEvents = true
			}
		default:
		}
	}
}

func e(x, y float64) int {
	return int(math.Pow(x, y))
}

func (f *Fluent) write(msg *msgToSend) error {
	var c net.Conn
	for i := 0; i < f.Config.MaxRetry; i++ {
		c = f.conn
		// Connect if needed
		if c == nil {
			f.muconn.Lock()
			if f.conn == nil {
				err := f.connect(msg.ctx)
				if err != nil {
					f.muconn.Unlock()

					if _, ok := err.(*ErrUnknownNetwork); ok {
						// do not retry on unknown network error
						break
					}
					waitTime := f.Config.RetryWait * e(defaultReconnectWaitIncreRate, float64(i-1))
					if waitTime > f.Config.MaxRetryWait {
						waitTime = f.Config.MaxRetryWait
					}
					waitDuration := time.Duration(waitTime) * time.Millisecond
					if deadline, hasDeadLine := msg.ctx.Deadline(); hasDeadLine && deadline.Before(time.Now().Add(waitDuration)) {
						// the context deadline is within the wait time, so after the sleep the deadline will have been
						// exceeded. It is a waste of time to wait on that.
						return context.DeadlineExceeded
					}
					time.Sleep(waitDuration)
					continue
				}
			}
			c = f.conn
			f.muconn.Unlock()
		}

		// We're connected, write msg
		t := f.Config.WriteTimeout
		var deadline time.Time
		if time.Duration(0) < t {
			deadline = time.Now().Add(t)
		}
		if ctxDeadline, hasDeadline := msg.ctx.Deadline(); hasDeadline && (deadline.IsZero() || ctxDeadline.Before(deadline)) {
			deadline = ctxDeadline
		}
		c.SetWriteDeadline(deadline)
		_, err := c.Write(msg.data)
		if err != nil {
			f.close(c)
		} else {
			// Acknowledgment check
			if msg.ack != "" {
				resp := &AckResp{}
				if f.Config.MarshalAsJSON {
					dec := json.NewDecoder(c)
					err = dec.Decode(resp)
				} else {
					r := msgp.NewReader(c)
					err = resp.DecodeMsg(r)
				}
				if err != nil || resp.Ack != msg.ack {
					f.close(c)
					continue
				}
			}
			return err
		}
	}

	return fmt.Errorf("fluent#write: failed to reconnect, max retry: %v", f.Config.MaxRetry)
}
