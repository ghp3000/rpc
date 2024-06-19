package rpc

import (
	"github.com/goccy/go-json"
	"github.com/vmihailenco/msgpack/v5"
)

type Codec interface {
	Name() string
	Marshal(i interface{}) ([]byte, error)
	Unmarshal(data []byte, i interface{}) error
}

type JsonCodec struct {
}

func (s *JsonCodec) Name() string {
	return "json"
}
func (s *JsonCodec) Marshal(i interface{}) ([]byte, error) {
	return json.Marshal(i)
}
func (s *JsonCodec) Unmarshal(data []byte, i interface{}) error {
	return json.Unmarshal(data, i)
}

type MsgPackCodec struct{}

func (s MsgPackCodec) Name() string {
	return "msgpack"
}
func (s MsgPackCodec) Marshal(i interface{}) ([]byte, error) {
	return msgpack.Marshal(i)
}
func (s MsgPackCodec) Unmarshal(data []byte, i interface{}) error {
	return msgpack.Unmarshal(data, i)
}
