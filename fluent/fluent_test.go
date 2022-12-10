package fluent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/tinylib/msgp/msgp"
)

func init() {
	// randomGenerator points to rand.Uint64 by default. Unfortunately, even when it's
	// seeded, it produces different values from time to time and thus is not fully
	// deterministic. This prevents writing stable tests for RequestAck config option.
	// Thus we need to change it to ensure the hashes are stable during tests.
	randomGenerator = func() uint64 {
		return 1
	}
}

func newTestDialer() *testDialer {
	return &testDialer{
		dialCh: make(chan *Conn),
	}
}

// testDialer is a stub for net.Dialer. It implements the Dial() method used by
// the logger to connect to Fluentd. It uses a *Conn channel to let the tests
// synchronize with calls to Dial() and let them define what each call to Dial()
// should return. This is especially useful for testing edge cases like
// transient connection failures.
// To help write test cases with succeeding or failing connection dialing, testDialer
// provides waitForNextDialing(). Any call to Dial() from the logger should be matched
// with a call to waitForNextDialing() in the test cases.
//
// For instance, to test an async logger that have to dial 4 times before succeeding,
// the test should look like this:
//
//	 d := newTestDialer() // Create a new stubbed dialer
//	 cfg := Config{
//	     Async: true,
//		   // ...
//	 }
//	 f := newWithDialer(cfg, d) // Create a fluent logger using the stubbed dialer
//	 f.EncodeAndPostData("tag_name", time.Unix(1482493046, 0), map[string]string{"foo": "bar"})
//
//	 d.waitForNextDialing(false, false) // 1st dialing attempt fails
//	 d.waitForNextDialing(false, false) // 2nd attempt fails too
//	 d.waitForNextDialing(false, false) // 3rd attempt fails too
//	 d.waitForNextDialing(true, false) // Finally the 4th attempt succeeds
//
// Note that in the above example, the logger operates in async mode. As such,
// a call to Post, PostWithTime or EncodeAndPostData is needed *before* calling
// waitForNextDialing(), as in async mode the logger initializes its connection
// lazily, in a separate goroutine.
// This also means non-async loggers can't be tested exactly the same way, as the
// dialing isn't done lazily but during the logger initialization. To test such
// case, you have to put the calls to newWithDialer() and to EncodeAndPostData()
// into their own goroutine. An example:
//
//	 d := newTestDialer() // Create a new stubbed dialer
//	 cfg := Config{
//	     Async: false,
//		   // ...
//	 }
//	 go func() {
//	     f := newWithDialer(cfg, d) // Create a fluent logger using the stubbed dialer
//	     f.Close()
//	 }()
//
//	 d.waitForNextDialing(false, false) // 1st dialing attempt fails
//	 d.waitForNextDialing(false, false) // 2nd attempt fails too
//	 d.waitForNextDialing(false, false) // 3rd attempt fails too
//	 d.waitForNextDialing(true, false) // Finally the 4th attempt succeeds
//
// Moreover, waitForNextDialing() returns a *Conn which extends net.Conn to provide testing
// facilities. For instance, you can call waitForNextWrite() on these connections, to
// specify how the next Conn.Write() call behaves (e.g. accept or reject it, or make a
// specific ack checksum available) and to assert what is sent to Fluentd (when the write
// is accepted). Again, any call to Write() on the logger side have to be matched with
// a call to waitForNextWrite() in the test cases.
//
// Here's a full example:
//
//	d := newTestDialer()
//	cfg := Config{Async: true}
//
//	f := newWithDialer(cfg, d)
//	f.EncodeAndPostData("tag_name", time.Unix(1482493046, 0), map[string]string{"foo": "bar"})
//
//	conn := d.waitForNextDialing(true, false) // Accept the dialing
//	conn.waitForNextWrite(false, "") // Discard the 1st attempt to write the message
//
//	conn := d.waitForNextDialing(true, false)
//	assertReceived(t, // t is *testing.T
//	    conn.waitForNextWrite(true, ""),
//	    `["tag_name",1482493046,{"foo":"bar"},{}]`)
//
//	f.EncodeAndPostData("something_else", time.Unix(1482493050, 0), map[string]string{"bar": "baz"})
//	assertReceived(t, // t is *testing.T
//	    conn.waitForNextWrite(true, ""),
//	    `["something_else",1482493050,{"bar":"baz"},{}]`)
//
// In this example, the 1st connection dialing succeeds but the 1st attempt to write the
// message is discarded. As the logger discards the connection whenever a message
// couldn't be written, it tries to re-dial and thus we need to accept the dialing again.
// Then the write is retried and accepted. When a second message is written, the write is
// accepted straightaway. Moreover, the messages written to the connections are asserted
// using assertReceived() to make sure the logger encodes the messages properly.
//
// Again, the example above is using async mode thus, calls to f and conn are running in
// the same goroutine. However, in sync mode, all calls to f.EncodeAndPostData() as well
// as the logger initialization shall be placed in a separate goroutine or the code
// allowing the dialing and writing attempts (eg. waitForNextDialing() & waitForNextWrite())
// would never be reached.
type testDialer struct {
	dialCh chan *Conn
}

// DialContext is the stubbed method called by the logger to establish the connection to
// Fluentd. It is paired with waitForNextDialing().
func (d *testDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	// It waits for a *Conn to be pushed into dialCh using waitForNextDialing(). When the
	// *Conn is nil, the Dial is deemed to fail.
	select {
	case conn := <-d.dialCh:
		if conn == nil {
			return nil, errors.New("failed to dial")
		}
		return conn, nil
	case <-ctx.Done():
		return nil, errors.New("failed to dial")
	}
}

// waitForNextDialing is the method used by test cases below to indicate whether the next
// dialing attempt made by the logger should succeed or not. See examples provided on
// testDialer docs.
func (d *testDialer) waitForNextDialing(accept bool, delayReads bool) *Conn {
	var conn *Conn
	if accept {
		conn = &Conn{
			nextWriteAttemptCh: make(chan nextWrite),
			writtenCh:          make(chan []byte),
		}

		if delayReads {
			conn.delayNextReadCh = make(chan struct{})
		}
	}

	d.dialCh <- conn
	return conn
}

// asserEqual asserts that actual and expected are equivalent, and otherwise
// marks the test as failed (t.Error). It uses reflect.DeepEqual internally.
func assertEqual(t *testing.T, actual, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("got: '%+v', expected: '%+v'", actual, expected)
	}
}

// assertReceived is used below by test cases to assert the content written to a *Conn
// matches an expected string. This is generally used in conjunction with
// Conn.waitForNextWrite().
func assertReceived(t *testing.T, rcv []byte, expected string) {
	t.Helper()
	assertEqual(t, string(rcv), expected)
}

// Conn extends net.Conn to add channels used to synchronise across goroutines, eg.
// between the goroutine doing the dialing (through newWithDialer in sync mode, or the
// first message logging in async mode) and the testing goroutine (making calls to
// Conn.waitForNextWrite()).
// This should be of low importance if you're not trying to understand/change how
// waitFor...() methods work. See examples provided in testDialer docs for higher
// level details.
type Conn struct {
	net.Conn
	buf           []byte
	writeDeadline time.Time
	// nextWriteAttemptCh is used by waitForNextWrite() to let Write() know if the next write
	// attempt should succeed or fail.
	nextWriteAttemptCh chan nextWrite
	// writtenCh is used by Write() to signal to waitForNextWrite() when a write
	// happened.
	writtenCh chan []byte
	// delayNextReadCh is used to delay next conn.Read() attempt when testing ack resp.
	delayNextReadCh chan struct{}
}

// nextWrite is the struct passed by Conn.waitForNextWrite() to Conn.Write() through
// Conn.nextWriteAttemptCh to let Write() know if it should accept or discard the next write
// operation and what ack checksum should be made readable from the connection.
// This should be of low importance if you're not trying to understand/change how
// waitFor...() methods work. See examples provided in testDialer docs for higher
// level details.
type nextWrite struct {
	accept bool
	ack    string
}

// waitForNextWrite is the method used to tell how the next write made by the logger
// should behave. It can either accept or discard the next write operation. Moreover
// an ack checksum can be passed such that the next Write operation will make it
// readable from the connection, as the logger will try to read it to ack the Write
// operation. See examples provided in testDialer docs.
func (c *Conn) waitForNextWrite(accept bool, ack string) []byte {
	c.nextWriteAttemptCh <- nextWrite{accept, ack}
	if accept {
		return <-c.writtenCh
	}
	return []byte{}
}

// Read is a stubbed version of net.Conn Read() that returns the ack checksum of the last
// Write operation.
func (c *Conn) Read(b []byte) (int, error) {
	if c.delayNextReadCh != nil {
		select {
		case _, ok := <-c.delayNextReadCh:
			if !ok {
				return 0, errors.New("connection has been closed")
			}
		default:
		}
	}

	copy(b, c.buf)
	return len(c.buf), nil
}

// Write is a stubbed version of net.Conn Write(). Its behavior is determined by the last
// call to waitForNextWrite(). See examples provided in testDialer docs.
func (c *Conn) Write(b []byte) (int, error) {
	next, ok := nextWrite{true, ""}, true
	if c.nextWriteAttemptCh != nil {
		next, ok = <-c.nextWriteAttemptCh
	}
	if !next.accept || !ok {
		return 0, errors.New("transient write failure")
	}

	// Write the acknowledgment to c.buf to make it available to subsequent
	// call to Read().
	c.buf = make([]byte, len(next.ack))
	copy(c.buf, next.ack)

	// Write the payload received to writtenCh to assert on it.
	if c.writtenCh != nil {
		c.writtenCh <- b
	}

	return len(b), nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}

func (c *Conn) Close() error {
	if c.delayNextReadCh != nil {
		close(c.delayNextReadCh)
	}

	return nil
}

func Test_New_itShouldUseDefaultConfigValuesIfNoOtherProvided(t *testing.T) {
	f, _ := New(Config{})
	assertEqual(t, f.Config.FluentPort, defaultPort)
	assertEqual(t, f.Config.FluentHost, defaultHost)
	assertEqual(t, f.Config.Timeout, defaultTimeout)
	assertEqual(t, f.Config.WriteTimeout, defaultWriteTimeout)
	assertEqual(t, f.Config.BufferLimit, defaultBufferLimit)
	assertEqual(t, f.Config.FluentNetwork, defaultNetwork)
	assertEqual(t, f.Config.FluentSocketPath, defaultSocketPath)
}

func Test_New_itShouldUseUnixDomainSocketIfUnixSocketSpecified(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported")
	}
	socketFile := "/tmp/fluent-logger-golang.sock"
	network := "unix"
	l, err := net.Listen(network, socketFile)
	if err != nil {
		t.Error(err)
		return
	}
	defer l.Close()

	f, err := New(Config{
		FluentNetwork:    network,
		FluentSocketPath: socketFile,
	})
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()
	assertEqual(t, f.Config.FluentNetwork, network)
	assertEqual(t, f.Config.FluentSocketPath, socketFile)

	socketFile = "/tmp/fluent-logger-golang-xxx.sock"
	network = "unixxxx"
	fUnknown, err := New(Config{
		FluentNetwork:    network,
		FluentSocketPath: socketFile,
	})
	if _, ok := err.(*ErrUnknownNetwork); !ok {
		t.Errorf("err type: %T", err)
	}
	if err == nil {
		t.Error(err)
		fUnknown.Close()
		return
	}
}

func Test_New_itShouldUseConfigValuesFromArguments(t *testing.T) {
	f, _ := New(Config{FluentPort: 6666, FluentHost: "foobarhost"})
	assertEqual(t, f.Config.FluentPort, 6666)
	assertEqual(t, f.Config.FluentHost, "foobarhost")
}

func Test_New_itShouldUseConfigValuesFromMashalAsJSONArgument(t *testing.T) {
	f, _ := New(Config{MarshalAsJSON: true})
	assertEqual(t, f.Config.MarshalAsJSON, true)
}

func Test_MarshalAsMsgpack(t *testing.T) {
	f := &Fluent{Config: Config{}}

	tag := "tag"
	data := map[string]string{
		"foo":  "bar",
		"hoge": "hoge",
	}
	tm := time.Unix(1267867237, 0)
	result, err := f.EncodeData(tag, tm, data)
	if err != nil {
		t.Error(err)
	}
	actual := string(result.data)

	// map entries are disordered in golang
	expected1 := "\x94\xA3tag\xD2K\x92\u001Ee\x82\xA3foo\xA3bar\xA4hoge\xA4hoge\x80"
	expected2 := "\x94\xA3tag\xD2K\x92\u001Ee\x82\xA4hoge\xA4hoge\xA3foo\xA3bar\x80"
	if actual != expected1 && actual != expected2 {
		t.Errorf("got %+v,\n         except %+v\n             or %+v", actual, expected1, expected2)
	}
}

func Test_SubSecondPrecision(t *testing.T) {
	// Setup the test subject
	fluent := &Fluent{
		Config: Config{
			SubSecondPrecision: true,
		},
	}
	fluent.conn = &Conn{}

	// Exercise the test subject
	timestamp := time.Unix(1267867237, 256)
	encodedData, err := fluent.EncodeData("tag", timestamp, map[string]string{
		"foo": "bar",
	})
	// Assert no encoding errors and that the timestamp has been encoded into
	// the message as expected.
	if err != nil {
		t.Error(err)
	}

	// 8 bytes timestamp can be represented using ext 8 or fixext 8
	expected1 := "\x94\xA3tag\xC7\x08\x00K\x92\u001Ee\x00\x00\x01\x00\x81\xA3foo\xA3bar\x80"
	expected2 := "\x94\xa3tag\xD7\x00K\x92\x1Ee\x00\x00\x01\x00\x81\xA3foo\xA3bar\x80"
	actual := string(encodedData.data)
	if actual != expected1 && actual != expected2 {
		t.Errorf("got %+v,\n         except %+v\n             or %+v", actual, expected1, expected2)
	}
}

func Test_MarshalAsJSON(t *testing.T) {
	f := &Fluent{Config: Config{MarshalAsJSON: true}}

	data := map[string]string{
		"foo":  "bar",
		"hoge": "hoge",
	}
	tm := time.Unix(1267867237, 0)
	result, err := f.EncodeData("tag", tm, data)
	if err != nil {
		t.Error(err)
	}
	// json.Encode marshals map keys in the order, so this expectation is safe
	expected := `["tag",1267867237,{"foo":"bar","hoge":"hoge"},{}]`
	actual := string(result.data)
	if actual != expected {
		t.Errorf("got %s, except %s", actual, expected)
	}
}

func TestJsonConfig(t *testing.T) {
	b, err := ioutil.ReadFile(`testdata/config.json`)
	if err != nil {
		t.Error(err)
	}
	var got Config
	expect := Config{
		FluentPort:         8888,
		FluentHost:         "localhost",
		FluentNetwork:      "tcp",
		FluentSocketPath:   "/var/tmp/fluent.sock",
		Timeout:            3000,
		WriteTimeout:       6000,
		BufferLimit:        10,
		RetryWait:          5,
		MaxRetry:           3,
		TagPrefix:          "fluent",
		Async:              false,
		ForceStopAsyncSend: false,
		MarshalAsJSON:      true,
	}

	err = json.Unmarshal(b, &got)
	if err != nil {
		t.Error(err)
	}

	assertEqual(t, got, expect)
}

func TestPostWithTime(t *testing.T) {
	testcases := map[string]Config{
		"with Async": {
			Async:         true,
			MarshalAsJSON: true,
			TagPrefix:     "acme",
		},
		"without Async": {
			Async:         false,
			MarshalAsJSON: true,
			TagPrefix:     "acme",
		},
	}

	for tcname, tc := range testcases {
		tc := tc
		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			d := newTestDialer()
			var f *Fluent
			defer func() {
				if f != nil {
					f.Close()
				}
			}()

			go func() {
				var err error
				if f, err = newWithDialer(tc, d); err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				_ = f.PostWithTime("tag_name", time.Unix(1482493046, 0), map[string]string{"foo": "bar"})
				_ = f.PostWithTime("tag_name", time.Unix(1482493050, 0), map[string]string{"fluentd": "is awesome"})
				_ = f.PostWithTime("tag_name", time.Unix(1634263200, 0),
					struct {
						Welcome string `msg:"welcome"`
						cannot  string
					}{"to use", "see me"})
			}()

			conn := d.waitForNextDialing(true, false)
			assertReceived(t,
				conn.waitForNextWrite(true, ""),
				`["acme.tag_name",1482493046,{"foo":"bar"},{}]`)

			assertReceived(t,
				conn.waitForNextWrite(true, ""),
				`["acme.tag_name",1482493050,{"fluentd":"is awesome"},{}]`)
			assertReceived(t,
				conn.waitForNextWrite(true, ""),
				`["acme.tag_name",1634263200,{"welcome":"to use"},{}]`)
		})
	}
}

func TestReconnectAndResendAfterTransientFailure(t *testing.T) {
	testcases := map[string]Config{
		"with Async": {
			Async:         true,
			MarshalAsJSON: true,
		},
		"without Async": {
			Async:         false,
			MarshalAsJSON: true,
		},
	}

	for tcname, tc := range testcases {
		tc := tc
		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			d := newTestDialer()
			var f *Fluent
			defer func() {
				if f != nil {
					f.Close()
				}
			}()

			go func() {
				var err error
				if f, err = newWithDialer(tc, d); err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				_ = f.EncodeAndPostData("tag_name", time.Unix(1482493046, 0), map[string]string{"foo": "bar"})
				_ = f.EncodeAndPostData("tag_name", time.Unix(1482493050, 0), map[string]string{"fluentd": "is awesome"})
			}()

			// Accept the first connection dialing and write.
			conn := d.waitForNextDialing(true, false)
			assertReceived(t,
				conn.waitForNextWrite(true, ""),
				`["tag_name",1482493046,{"foo":"bar"},{}]`)

			// The next write will fail and the next connection dialing will be dropped
			// to test if the logger is reconnecting as expected.
			conn.waitForNextWrite(false, "")
			d.waitForNextDialing(false, false)

			// Next, we allow a new connection to be established and we allow the last message to be written.
			conn = d.waitForNextDialing(true, false)
			assertReceived(t,
				conn.waitForNextWrite(true, ""),
				`["tag_name",1482493050,{"fluentd":"is awesome"},{}]`)
		})
	}
}

func timeout(t *testing.T, duration time.Duration, fn func(), reason string) {
	done := make(chan struct{})
	go func() {
		fn()
		done <- struct{}{}
	}()

	select {
	case <-time.After(duration):
		t.Fatalf("time out after %s: %s", duration.String(), reason)
	case <-done:
		return
	}
}

func TestCloseOnFailingAsyncConnect(t *testing.T) {
	testcases := map[string]Config{
		"with ForceStopAsyncSend and with RequestAck": {
			Async:              true,
			ForceStopAsyncSend: true,
			RequestAck:         true,
		},
		"with ForceStopAsyncSend and without RequestAck": {
			Async:              true,
			ForceStopAsyncSend: true,
			RequestAck:         false,
		},
		"without ForceStopAsyncSend and with RequestAck": {
			Async:              true,
			ForceStopAsyncSend: false,
			RequestAck:         true,
		},
		"without ForceStopAsyncSend and without RequestAck": {
			Async:              true,
			ForceStopAsyncSend: false,
			RequestAck:         false,
		},
	}

	for tcname, tc := range testcases {
		tc := tc
		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			d := newTestDialer()
			f, err := newWithDialer(tc, d)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			timeout(t, 1*time.Second, func() { f.Close() }, "failed to close the logger")
		})
	}
}

func ackRespMsgp(t *testing.T, ack string) string {
	msg := AckResp{ack}
	buf := &bytes.Buffer{}
	ackW := msgp.NewWriter(buf)
	if err := msg.EncodeMsg(ackW); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ackW.Flush()
	return buf.String()
}

func TestNoPanicOnAsyncClose(t *testing.T) {
	testcases := []struct {
		name        string
		config      Config
		shouldError bool
	}{
		{
			name: "Channel closed before write",
			config: Config{
				Async: true,
			},
			shouldError: true,
		},
		{
			name: "Channel not closed at all",
			config: Config{
				Async: true,
			},
			shouldError: false,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := newTestDialer()
			f, err := newWithDialer(tc.config, d)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tc.shouldError {
				f.Close()
			}
			err = f.EncodeAndPostData("tag_name", time.Unix(1482493046, 0), map[string]string{"foo": "bar"})
			if tc.shouldError {
				assertEqual(t, err, fmt.Errorf("fluent#appendBuffer: Logger already closed"))
			} else {
				assertEqual(t, err, nil)
			}
		})
	}
}

func TestNoPanicOnAsyncMultipleClose(t *testing.T) {
	config := Config{
		Async: true,
	}
	d := newTestDialer()
	f, err := newWithDialer(config, d)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	f.Close()
	f.Close()
}

func TestCloseOnFailingAsyncReconnect(t *testing.T) {
	testcases := map[string]Config{
		"with RequestAck": {
			Async:              true,
			ForceStopAsyncSend: true,
			RequestAck:         true,
		},
		"without RequestAck": {
			Async:              true,
			ForceStopAsyncSend: true,
			RequestAck:         false,
		},
	}

	for tcname, tc := range testcases {
		tc := tc
		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			d := newTestDialer()
			f, err := newWithDialer(tc, d)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Send a first message successfully.
			_ = f.EncodeAndPostData("tag_name", time.Unix(1482493046, 0), map[string]string{"foo": "bar"})
			conn := d.waitForNextDialing(true, false)
			conn.waitForNextWrite(true, ackRespMsgp(t, "dgxdWAAAAAABAAAAAAAAAA=="))

			// Then try to send one during a transient connection failure.
			_ = f.EncodeAndPostData("tag_name", time.Unix(1482493046, 0), map[string]string{"bar": "baz"})
			conn.waitForNextWrite(false, "")

			// And add some more logs to the log buffer.
			_ = f.EncodeAndPostData("tag_name", time.Unix(1482493046, 0), map[string]string{"acme": "corporation"})

			// But close the logger before it got sent. This is expected to not block.
			timeout(t, 60*time.Second, func() { f.Close() }, "failed to close the logger")
		})
	}
}

func TestCloseWhileWaitingForAckResponse(t *testing.T) {
	t.Parallel()

	d := newTestDialer()
	f, err := newWithDialer(Config{
		Async:              true,
		RequestAck:         true,
		ForceStopAsyncSend: true,
	}, d)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	_ = f.EncodeAndPostData("tag_name", time.Unix(1482493046, 0), map[string]string{"foo": "bar"})
	conn := d.waitForNextDialing(true, true)
	conn.waitForNextWrite(true, ackRespMsgp(t, "dgxdWAAAAAABAAAAAAAAAA=="))

	// Test if the logger can really by closed while the client waits for a ack message.
	timeout(t, 30*time.Second, func() {
		f.Close()
	}, "failed to close the logger")
}

func TestSyncWriteAfterCloseFails(t *testing.T) {
	d := newTestDialer()

	go func() {
		f, err := newWithDialer(Config{Async: false}, d)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		err = f.PostWithTime("tag_name", time.Unix(1482493046, 0), map[string]string{"foo": "bar"})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		err = f.Close()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Now let's post some event after Fluent.Close().
		err = f.PostWithTime("tag_name", time.Unix(1482493050, 0), map[string]string{"foo": "buzz"})

		// The event submission must fail,
		if err == nil {
			t.Error("expected an error")
		}

		// and also must keep Fluentd closed.
		if f.closed != true {
			t.Error("expected Fluentd to be kept closed")
		}
	}()

	conn := d.waitForNextDialing(true, false)
	conn.waitForNextWrite(true, "")
}

func Benchmark_PostWithShortMessage(b *testing.B) {
	b.StopTimer()
	d := newTestDialer()
	f, err := newWithDialer(Config{}, d)
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	data := map[string]string{"message": "Hello World"}
	for i := 0; i < b.N; i++ {
		if err := f.Post("tag", data); err != nil {
			panic(err)
		}
	}
}

func Benchmark_PostWithShortMessageMarshalAsJSON(b *testing.B) {
	b.StopTimer()
	f, err := New(Config{MarshalAsJSON: true})
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	data := map[string]string{"message": "Hello World"}
	for i := 0; i < b.N; i++ {
		if err := f.Post("tag", data); err != nil {
			panic(err)
		}
	}
}

func Benchmark_LogWithChunks(b *testing.B) {
	b.StopTimer()
	f, err := New(Config{})
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	data := map[string]string{"msg": "sdfsdsdfdsfdsddddfsdfsdsdfdsfdsddddfsdfsdsdfdsfdsddddfsdfsdsdfdsfdsddddfsdfsdsdfdsfdsddddfsdfsdsdfdsfdsddddfsdfsdsdfdsfdsddddfsdfsdsdfdsfdsddddf"}
	for i := 0; i < b.N; i++ {
		if err := f.Post("tag", data); err != nil {
			panic(err)
		}
	}
}

func Benchmark_PostWithStruct(b *testing.B) {
	b.StopTimer()
	f, err := New(Config{})
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	data := struct {
		Name string `msg:"msgnamename"`
	}{
		"john smith",
	}
	for i := 0; i < b.N; i++ {
		if err := f.Post("tag", data); err != nil {
			panic(err)
		}
	}
}

func Benchmark_PostWithStructTaggedAsCodec(b *testing.B) {
	b.StopTimer()
	f, err := New(Config{})
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	data := struct {
		Name string `codec:"codecname"`
	}{
		"john smith",
	}
	for i := 0; i < b.N; i++ {
		if err := f.Post("tag", data); err != nil {
			panic(err)
		}
	}
}

func Benchmark_PostWithStructWithoutTag(b *testing.B) {
	b.StopTimer()
	f, err := New(Config{})
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	data := struct {
		Name string
	}{
		"john smith",
	}
	for i := 0; i < b.N; i++ {
		if err := f.Post("tag", data); err != nil {
			panic(err)
		}
	}
}

func Benchmark_PostWithMapString(b *testing.B) {
	b.StopTimer()
	f, err := New(Config{})
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	data := map[string]string{
		"foo": "bar",
	}
	for i := 0; i < b.N; i++ {
		if err := f.Post("tag", data); err != nil {
			panic(err)
		}
	}
}

func Benchmark_PostWithMsgpMarshaler(b *testing.B) {
	b.StopTimer()
	f, err := New(Config{})
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	data := &TestMessage{Foo: "bar"}
	for i := 0; i < b.N; i++ {
		if err := f.Post("tag", data); err != nil {
			panic(err)
		}
	}
}

func Benchmark_PostWithMapSlice(b *testing.B) {
	b.StopTimer()
	f, err := New(Config{})
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	data := map[string][]int{
		"foo": {1, 2, 3},
	}
	for i := 0; i < b.N; i++ {
		if err := f.Post("tag", data); err != nil {
			panic(err)
		}
	}
}

func Benchmark_PostWithMapStringAndTime(b *testing.B) {
	b.StopTimer()
	f, err := New(Config{})
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	data := map[string]string{
		"foo": "bar",
	}
	tm := time.Now()
	for i := 0; i < b.N; i++ {
		if err := f.PostWithTime("tag", tm, data); err != nil {
			panic(err)
		}
	}
}
