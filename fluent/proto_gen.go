package fluent

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *Forward) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "tag":
			z.Tag, err = dc.ReadString()
			if err != nil {
				return
			}
		case "entries":
			var xsz uint32
			xsz, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.Entries) >= int(xsz) {
				z.Entries = z.Entries[:xsz]
			} else {
				z.Entries = make([]Entry, xsz)
			}
			for xvk := range z.Entries {
				var isz uint32
				isz, err = dc.ReadMapHeader()
				if err != nil {
					return
				}
				for isz > 0 {
					isz--
					field, err = dc.ReadMapKeyPtr()
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "time":
						z.Entries[xvk].Time, err = dc.ReadInt64()
						if err != nil {
							return
						}
					case "record":
						z.Entries[xvk].Record, err = dc.ReadIntf()
						if err != nil {
							return
						}
					default:
						err = dc.Skip()
						if err != nil {
							return
						}
					}
				}
			}
		case "option":
			z.Option, err = dc.ReadIntf()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Forward) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteMapHeader(3)
	if err != nil {
		return
	}
	err = en.WriteString("tag")
	if err != nil {
		return
	}
	err = en.WriteString(z.Tag)
	if err != nil {
		return
	}
	err = en.WriteString("entries")
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Entries)))
	if err != nil {
		return
	}
	for xvk := range z.Entries {
		err = en.WriteMapHeader(2)
		if err != nil {
			return
		}
		err = en.WriteString("time")
		if err != nil {
			return
		}
		err = en.WriteInt64(z.Entries[xvk].Time)
		if err != nil {
			return
		}
		err = en.WriteString("record")
		if err != nil {
			return
		}
		err = en.WriteIntf(z.Entries[xvk].Record)
		if err != nil {
			return
		}
	}
	err = en.WriteString("option")
	if err != nil {
		return
	}
	err = en.WriteIntf(z.Option)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Forward) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendMapHeader(o, 3)
	o = msgp.AppendString(o, "tag")
	o = msgp.AppendString(o, z.Tag)
	o = msgp.AppendString(o, "entries")
	o = msgp.AppendArrayHeader(o, uint32(len(z.Entries)))
	for xvk := range z.Entries {
		o = msgp.AppendMapHeader(o, 2)
		o = msgp.AppendString(o, "time")
		o = msgp.AppendInt64(o, z.Entries[xvk].Time)
		o = msgp.AppendString(o, "record")
		o, err = msgp.AppendIntf(o, z.Entries[xvk].Record)
		if err != nil {
			return
		}
	}
	o = msgp.AppendString(o, "option")
	o, err = msgp.AppendIntf(o, z.Option)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Forward) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "tag":
			z.Tag, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "entries":
			var xsz uint32
			xsz, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Entries) >= int(xsz) {
				z.Entries = z.Entries[:xsz]
			} else {
				z.Entries = make([]Entry, xsz)
			}
			for xvk := range z.Entries {
				var isz uint32
				isz, bts, err = msgp.ReadMapHeaderBytes(bts)
				if err != nil {
					return
				}
				for isz > 0 {
					isz--
					field, bts, err = msgp.ReadMapKeyZC(bts)
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "time":
						z.Entries[xvk].Time, bts, err = msgp.ReadInt64Bytes(bts)
						if err != nil {
							return
						}
					case "record":
						z.Entries[xvk].Record, bts, err = msgp.ReadIntfBytes(bts)
						if err != nil {
							return
						}
					default:
						bts, err = msgp.Skip(bts)
						if err != nil {
							return
						}
					}
				}
			}
		case "option":
			z.Option, bts, err = msgp.ReadIntfBytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *Forward) Msgsize() (s int) {
	s = msgp.MapHeaderSize + msgp.StringPrefixSize + 3 + msgp.StringPrefixSize + len(z.Tag) + msgp.StringPrefixSize + 7 + msgp.ArrayHeaderSize
	for xvk := range z.Entries {
		s += msgp.MapHeaderSize + msgp.StringPrefixSize + 4 + msgp.Int64Size + msgp.StringPrefixSize + 6 + msgp.GuessSize(z.Entries[xvk].Record)
	}
	s += msgp.StringPrefixSize + 6 + msgp.GuessSize(z.Option)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Message) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "tag":
			z.Tag, err = dc.ReadString()
			if err != nil {
				return
			}
		case "time":
			z.Time, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "record":
			z.Record, err = dc.ReadIntf()
			if err != nil {
				return
			}
		case "option":
			z.Option, err = dc.ReadIntf()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Message) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteMapHeader(4)
	if err != nil {
		return
	}
	err = en.WriteString("tag")
	if err != nil {
		return
	}
	err = en.WriteString(z.Tag)
	if err != nil {
		return
	}
	err = en.WriteString("time")
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Time)
	if err != nil {
		return
	}
	err = en.WriteString("record")
	if err != nil {
		return
	}
	err = en.WriteIntf(z.Record)
	if err != nil {
		return
	}
	err = en.WriteString("option")
	if err != nil {
		return
	}
	err = en.WriteIntf(z.Option)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Message) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendMapHeader(o, 4)
	o = msgp.AppendString(o, "tag")
	o = msgp.AppendString(o, z.Tag)
	o = msgp.AppendString(o, "time")
	o = msgp.AppendInt64(o, z.Time)
	o = msgp.AppendString(o, "record")
	o, err = msgp.AppendIntf(o, z.Record)
	if err != nil {
		return
	}
	o = msgp.AppendString(o, "option")
	o, err = msgp.AppendIntf(o, z.Option)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Message) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "tag":
			z.Tag, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "time":
			z.Time, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "record":
			z.Record, bts, err = msgp.ReadIntfBytes(bts)
			if err != nil {
				return
			}
		case "option":
			z.Option, bts, err = msgp.ReadIntfBytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *Message) Msgsize() (s int) {
	s = msgp.MapHeaderSize + msgp.StringPrefixSize + 3 + msgp.StringPrefixSize + len(z.Tag) + msgp.StringPrefixSize + 4 + msgp.Int64Size + msgp.StringPrefixSize + 6 + msgp.GuessSize(z.Record) + msgp.StringPrefixSize + 6 + msgp.GuessSize(z.Option)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Entry) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "time":
			z.Time, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "record":
			z.Record, err = dc.ReadIntf()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z Entry) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteMapHeader(2)
	if err != nil {
		return
	}
	err = en.WriteString("time")
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Time)
	if err != nil {
		return
	}
	err = en.WriteString("record")
	if err != nil {
		return
	}
	err = en.WriteIntf(z.Record)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Entry) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendMapHeader(o, 2)
	o = msgp.AppendString(o, "time")
	o = msgp.AppendInt64(o, z.Time)
	o = msgp.AppendString(o, "record")
	o, err = msgp.AppendIntf(o, z.Record)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Entry) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "time":
			z.Time, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "record":
			z.Record, bts, err = msgp.ReadIntfBytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z Entry) Msgsize() (s int) {
	s = msgp.MapHeaderSize + msgp.StringPrefixSize + 4 + msgp.Int64Size + msgp.StringPrefixSize + 6 + msgp.GuessSize(z.Record)
	return
}
