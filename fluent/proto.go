//go:generate msgp

package fluent

type Entry struct {
	Time   int64       `msg:"time"`
	Record interface{} `msg:"record"`
}

type Forward struct {
	Tag     string      `msg:"tag"`
	Entries []Entry     `msg:"entries"`
	Option  interface{} `msg:"option"`
}

type Message struct {
	Tag    string      `msg:"tag"`
	Time   int64       `msg:"time"`
	Record interface{} `msg:"record"`
	Option interface{} `msg:"option"`
}
