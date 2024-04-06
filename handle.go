package rpc

import (
	"time"
)

type handle func(msg *Message) (response *Message)

type Handles struct {
	routes    map[string]handle
	noMethod  func(msg *Message) *Message
	onRecover func(*Client, any)
	onErr     func(c *Client, err error)
}

func NewHandles() *Handles {
	return &Handles{routes: make(map[string]handle, 0)}
}
func (s *Handles) Register(method string, f handle) {
	s.routes[method] = f
}
func (s *Handles) Do(c *Client, msg *Message) {
	defer func() {
		if err := recover(); err != nil {
			if s.onRecover != nil {
				s.onRecover(c, err)
			}
		}
	}()
	t := time.Now()
	h, ok := s.routes[msg.Method]
	if !ok {
		if g := s.invalidMethod(msg); g != nil {
			if err := c.Send(g); err != nil {
				s.doErr(c, err)
			}
		} else {
			msg.Code = ErrCodeMethodNotFound
			if err := c.Send(msg); err != nil {
				s.doErr(c, err)
			}
		}
		return
	}
	if h == nil {
		msg.Code = ErrCodeInternalError
		if err := c.Send(msg); err != nil {
			s.doErr(c, err)
		}
		return
	}
	v := h(msg)
	if v == nil {
		return
	}
	v.T = uint32(time.Since(t).Milliseconds())
	if err := c.Send(v); err != nil {
		s.doErr(c, err)
	}
}
func (s *Handles) invalidMethod(msg *Message) *Message {
	if s.noMethod != nil {
		return s.noMethod(msg)
	}
	return nil
}
func (s *Handles) OnError(f func(c *Client, err error)) {
	s.onErr = f
}
func (s *Handles) OnPanic(f func(c *Client, err any)) {
	s.onRecover = f
}
func (s *Handles) doErr(c *Client, err error) {
	if s.onErr != nil {
		s.onErr(c, err)
	}
}
