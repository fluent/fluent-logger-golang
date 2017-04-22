//go:generate msgp

package fluent

import (
	"time"

	"github.com/tinylib/msgp/msgp"
)

//msgp:tuple Entry
type Entry struct {
	Time   int64       `msg:"time"`
	Record interface{} `msg:"record"`
}

//msgp:tuple Forward
type Forward struct {
	Tag     string      `msg:"tag"`
	Entries []Entry     `msg:"entries"`
	Option  interface{} `msg:"option"`
}

//msgp:tuple Message
type Message struct {
	Tag    string      `msg:"tag"`
	Time   EventTime   `msg:"time,extension"`
	Record interface{} `msg:"record"`
	Option interface{} `msg:"option"`
}

type EventTime time.Time

func init() {
	msgp.RegisterExtension(0, func() msgp.Extension { return new(EventTime) })
}

func (t *EventTime) ExtensionType() int8 { return 0 }

func (t *EventTime) Len() int { return 8 }

func (t *EventTime) MarshalBinaryTo(b []byte) error {
	// For more info on fluentd protocol and msgpack serialization and
	// extensions:
	// * https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1
	// * https://github.com/msgpack/msgpack/blob/master/spec.md
	// * https://github.com/tinylib/msgp/wiki/Using-Extensions

	// Unbox to Golang time
	goTime := time.Time(*t)

	// There's no support for timezones in fluentd's protocol for EventTime.
	// Assume UTC.
	utc := goTime.UTC()

	// Warning! This operation is lossy.
	sec := int32(utc.Unix())
	nsec := utc.Nanosecond()

	// Write the 4 bytes for the second component and 4 bytes for the nanosecond
	// component.
	b[0] = byte(sec >> 24)
	b[1] = byte(sec >> 16)
	b[2] = byte(sec >> 8)
	b[3] = byte(sec)
	b[4] = byte(nsec >> 24)
	b[5] = byte(nsec >> 16)
	b[6] = byte(nsec >> 8)
	b[7] = byte(nsec)

	return nil
}

func (t *EventTime) UnmarshalBinary(b []byte) error {
	return nil
}
