package rpc

import (
	"errors"
	"fmt"
	"github.com/goccy/go-json"
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
	Method   string          `json:"f" msgpack:"f"`                     //函数名
	Sequence uint32          `json:"s" msgpack:"s"`                     //包序号
	Code     uint8           `json:"c,omitempty" msgpack:"c,omitempty"` //状态码
	Msg      string          `json:"m,omitempty" msgpack:"m,omitempty"` //状态文本
	T        uint32          `json:"t,omitempty" msgpack:"t,omitempty"` //执行耗时
	Data     json.RawMessage `json:"d,omitempty" msgpack:"d,omitempty"` //数据
	codec    Codec           `msgpack:"-"`
}

func NewMsgPackFromBytes(buf []byte, codec Codec) (*Message, error) {
	var v Message
	err := codec.Unmarshal(buf, &v)
	if err != nil {
		return nil, err
	}
	v.codec = codec
	return &v, err
}
func (m *Message) Marshal() ([]byte, error) {
	return m.getCodec().Marshal(m)
}
func (m *Message) SetData(v interface{}) error {
	buf, err := m.getCodec().Marshal(v)
	if err != nil {
		return err
	}
	m.Data = buf
	return nil
}
func (m *Message) UnmarshalData(v interface{}) error {
	return m.getCodec().Unmarshal(m.Data, v)
}
func (m *Message) getCodec() Codec {
	if m.codec == nil {
		return &MsgPackCodec{}
	}
	return m.codec
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
func (m *Message) SetError(e string) {
	m.Code = ErrCodeStandard
	m.Msg = e
}
