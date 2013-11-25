package fluent

import (
	"github.com/ugorji/go/codec"
	"math"
)

var (
	mh codec.MsgpackHandle
)

func e(x, y float64) int {
	return int(math.Pow(x, y))
}

func toMsgpack(val interface{}) (packed []byte, err error) {
	enc := codec.NewEncoderBytes(&packed, &mh)
	err = enc.Encode(val)
	return
}
