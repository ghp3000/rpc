package rpc

import (
	"errors"
	"fmt"
	"github.com/goccy/go-json"
	"github.com/vmihailenco/msgpack/v5"
)

type CodecType int8

const (
	CodecTypeMsgPack CodecType = 0
	CodecTypeJson    CodecType = 1
)

const (
	ErrCodeMethodNotFound byte = 255
	ErrCodeInternalError  byte = 254
	ErrCodeStandard       byte = 1
	ErrCodeNone                = 0
)

var (
	ErrMethodNotFound = errors.New("method is not found")
	ErrInternalError  = errors.New("no handle")
	ErrTimeout        = errors.New("timeout")
	ErrConnNil        = errors.New("client conn is invalid")
	ErrOffline        = errors.New("not online")
)

type Message struct {
	Method      string             `json:"f" msgpack:"f" validate:"required"`                       //函数名
	Sequence    uint32             `json:"s" msgpack:"s" validate:"required"`                       //包序号
	Reply       bool               `json:"r,omitempty" msgpack:"r,omitempty" validate:"omitempty"`  //是否是返回值
	Code        uint8              `json:"c,omitempty" msgpack:"c,omitempty"  validate:"omitempty"` //状态码
	Msg         string             `json:"m,omitempty" msgpack:"m,omitempty"  validate:"omitempty"` //状态文本
	T           uint32             `json:"t,omitempty" msgpack:"t,omitempty"  validate:"omitempty"` //执行耗时
	DataJson    json.RawMessage    `json:"d,omitempty" msgpack:"-"  validate:"omitempty"`           //数据
	DataMsgpack msgpack.RawMessage `json:"-" msgpack:"d,omitempty"  validate:"omitempty"`           //数据
	codec       CodecType          `msgpack:"-"`
}

// NewMessage typ=0 msgpack typ=1 json
func NewMessage(typ CodecType) *Message {
	return &Message{codec: typ}
}
func NewMessageFromBytes(buf []byte) (*Message, error) {
	if len(buf) == 0 {
		return nil, errors.New("empty data")
	}
	var typ CodecType
	if buf[0] == 0x7b {
		typ = CodecTypeJson
	} else {
		typ = CodecTypeMsgPack
	}
	var err error
	var v Message
	if typ == CodecTypeJson {
		err = json.Unmarshal(buf, &v)
	} else if typ == CodecTypeMsgPack {
		err = msgpack.Unmarshal(buf, &v)
	}
	if err != nil {
		return nil, err
	}
	v.codec = typ
	return &v, err
}
func (m *Message) Marshal() ([]byte, error) {
	if m.codec == CodecTypeJson {
		return json.Marshal(m)
	}

	return msgpack.Marshal(m)
}
func (m *Message) Data() []byte {
	if m.codec == CodecTypeJson {
		return m.DataJson
	}
	return m.DataMsgpack
}
func (m *Message) SetData(v interface{}) error {
	if m.codec == CodecTypeJson {
		buf, err := json.Marshal(v)
		if err != nil {
			return err
		}
		m.DataJson = buf
	} else {
		buf, err := msgpack.Marshal(v)
		if err != nil {
			return err
		}
		m.DataMsgpack = buf
	}
	return nil
}

// ShouldSetData 置入data数据,忽略错误并返回对象,方便链式调用
func (m *Message) ShouldSetData(v interface{}) *Message {
	_ = m.SetData(v)
	return m
}
func (m *Message) UnmarshalData(v interface{}) error {
	if m.codec == CodecTypeJson {
		return json.Unmarshal(m.DataJson, v)
	}

	return msgpack.Unmarshal(m.DataMsgpack, v)
}
func (m *Message) Error() error {
	switch m.Code {
	case ErrCodeNone:
		return nil
	case ErrCodeMethodNotFound:
		return ErrMethodNotFound
	case ErrCodeInternalError:
		return ErrInternalError
	default:
		if len(m.Msg) > 0 {
			return errors.New(m.Msg)
		} else {
			return fmt.Errorf("response error code=%d", m.Code)
		}
	}
}

/*
	func (m *Message) SetRawData(data []byte) *Message {
		if m.codec == CodecTypeJson {
			m.DataJson = data
		} else {
			m.DataMsgpack = data
		}
		return m
	}
*/

func (m *Message) SetError(code byte, e string) *Message {
	m.Msg = e
	m.Code = code
	return m
}
func (m *Message) ToJson() ([]byte, error) {
	if m.codec == CodecTypeJson {
		return json.Marshal(m)
	} else {
		var vv interface{}
		err := m.UnmarshalData(&vv)
		if err != nil {
			return nil, err
		}
		var v = make(map[string]interface{})
		v["f"] = m.Method
		v["s"] = m.Sequence
		v["r"] = m.Reply
		v["c"] = m.Code
		v["m"] = m.Msg
		v["t"] = m.T
		v["d"] = vv
		return json.Marshal(v)
	}
}
func (m *Message) ToMsgpack() ([]byte, error) {
	if m.codec == CodecTypeMsgPack {
		return msgpack.Marshal(m)
	} else {
		var vv interface{}
		err := m.UnmarshalData(&vv)
		if err != nil {
			return nil, err
		}
		var v = make(map[string]interface{})
		v["f"] = m.Method
		v["s"] = m.Sequence
		v["r"] = m.Reply
		v["c"] = m.Code
		v["m"] = m.Msg
		v["t"] = m.T
		v["d"] = vv
		return msgpack.Marshal(v)
	}
}
func (m *Message) CodecName() string {
	if m.codec == CodecTypeJson {
		return "json"
	} else if m.codec == CodecTypeMsgPack {
		return "msgpack"
	}
	return "unknown codec"
}
