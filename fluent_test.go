package fluent

import (
	"github.com/bmizerany/assert"
	"testing"
)

func Test_New_itShouldUseDefaultConfigValuesIfNoOtherProvided(t *testing.T) {
	f, _ := New(Config{})
	assert.Equal(t, f.Config.FluentPort, defaultPort)
	assert.Equal(t, f.Config.FluentHost, defaultHost)
}

func Test_New_itShouldUseConfigValuesFromArguments(t *testing.T) {
	f, _ := New(Config{FluentPort: 80, FluentHost: "foobarhost"})
	assert.Equal(t, f.Config.FluentPort, 80)
	assert.Equal(t, f.Config.FluentHost, "foobarhost")
}

func Benchmark_PostWithShortMessage(b *testing.B) {
	b.StopTimer()
	f, err := New(Config{})
	if err != nil {
		panic(err)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		f.Post("tag", "Hello World")
	}
}
