package fluent

import (
	"bytes"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/bmizerany/assert"
)

const (
	RECV_BUF_LEN = 1024
)

// Conn is io.WriteCloser
type Conn struct {
	bytes.Buffer
}

func (c *Conn) Close() error {
	return nil
}

func init() {
	numProcs := runtime.NumCPU()
	if numProcs < 2 {
		numProcs = 2
	}
	runtime.GOMAXPROCS(numProcs)

	listener, err := net.Listen("tcp", "0.0.0.0:6666")
	if err != nil {
		println("error listening:", err.Error())
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				println("Error accept:", err.Error())
				return
			}
			go EchoFunc(conn)
		}
	}()
}

func EchoFunc(conn net.Conn) {
	for {
		buf := make([]byte, RECV_BUF_LEN)
		n, err := conn.Read(buf)
		if err != nil {
			println("Error reading:", err.Error())
			return
		}
		println("received ", n, " bytes of data =", string(buf))
	}
}

func Test_New_itShouldUseDefaultConfigValuesIfNoOtherProvided(t *testing.T) {
	f, _ := New(Config{})
	assert.Equal(t, f.Config.FluentPort, defaultPort)
	assert.Equal(t, f.Config.FluentHost, defaultHost)
	assert.Equal(t, f.Config.Timeout, defaultTimeout)
	assert.Equal(t, f.Config.BufferLimit, defaultBufferLimit)
	assert.Equal(t, f.Config.FluentNetwork, defaultNetwork)
	assert.Equal(t, f.Config.FluentSocketPath, defaultSocketPath)
}

func Test_New_itShouldUseUnixDomainSocketIfUnixSocketSpecified(t *testing.T) {

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
		FluentSocketPath: socketFile})
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()
	assert.Equal(t, f.Config.FluentNetwork, network)
	assert.Equal(t, f.Config.FluentSocketPath, socketFile)

	socketFile = "/tmp/fluent-logger-golang-xxx.sock"
	network = "unixxxx"
	fUnknown, err := New(Config{
		FluentNetwork:    network,
		FluentSocketPath: socketFile})
	if _, ok := err.(net.UnknownNetworkError); !ok {
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
	assert.Equal(t, f.Config.FluentPort, 6666)
	assert.Equal(t, f.Config.FluentHost, "foobarhost")
}

func Test_send_WriteBufferToConn(t *testing.T) {
	f := &Fluent{Config: Config{}}

	buf := &Conn{}
	f.conn = buf

	msg := "This is test writing."
	bmsg := []byte(msg)
	f.buf = append(f.buf, bmsg...)

	err := f.send(f.buf)
	if err != nil {
		t.Error(err)
	}

	rcv := buf.String()
	if rcv != msg {
		t.Errorf("got %s, except %s", rcv, msg)
	}
}

func Benchmark_PostWithShortMessage(b *testing.B) {
	b.StopTimer()
	f, err := New(Config{})
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
