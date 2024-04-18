package rpc

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type AsyncResponse struct {
	Ch     chan Message
	Seq    uint32
	Client *Client
}

// Close 异步收取结束后必须调用,否则资源不会回收
func (s *AsyncResponse) Close() {
	close(s.Ch)
	if s.Client != nil {
		s.Client.wait.Delete(s.Seq)
	}
}

type Client struct {
	seq          uint32
	conn         net.Conn
	lock         sync.Mutex
	wait         sync.Map
	extra        interface{}
	reconnect    uint32
	delay        time.Duration
	dialer       func() (net.Conn, error)
	running      bool
	online       atomic.Bool
	handle       *Handles
	onConnect    func(c *Client)
	onDisconnect func(c *Client)
	codec        Codec
}

func NewClientWithDialer(dialer func() (net.Conn, error), reconnect bool, delay time.Duration, codec Codec) *Client {
	c := &Client{conn: nil, delay: delay, codec: codec, dialer: dialer, reconnect: func(r bool) uint32 {
		if r {
			return 1
		} else {
			return 0
		}
	}(reconnect)}
	return c
}

func NewClient(conn net.Conn, codec Codec) *Client {
	c := &Client{conn: conn, codec: codec, online: atomic.Bool{}}
	c.online.Store(true)
	return c
}
func (c *Client) Run() {
	go c.loop()
}
func (c *Client) Send(msg *Message) error {
	if c.conn == nil {
		return ErrConnNil
	}
	buf, err := msg.Marshal()
	if err != nil {
		return err
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	b := bytes.NewBuffer(nil)
	if err = binary.Write(b, binary.LittleEndian, uint32(len(buf))); err != nil {
		return err
	}
	if _, err = c.conn.Write(b.Bytes()); err != nil {
		return err
	}
	if _, err = c.conn.Write(buf); err != nil {
		return err
	}
	return nil
}
func (c *Client) iter() (seq uint32) {
	return atomic.AddUint32(&c.seq, 1)
}
func (c *Client) loop() {
	var err error
	c.running = true
	for c.running {
		c.conn, err = c.dialer()
		if err != nil {
			if atomic.LoadUint32(&c.reconnect) == 0 {
				c.Close()
				return
			}
			time.Sleep(c.delay)
			atomic.AddUint32(&c.reconnect, 1)
			continue
		}
		c.recvLoop()
		if c.onDisconnect != nil {
			c.onDisconnect(c)
		}
	}
}

// recvLoop 客户端需要循环读取网络数据
func (c *Client) recvLoop() {
	c.online.Store(true)
	defer c.online.Store(false)

	if c.onConnect != nil {
		go c.onConnect(c)
	}

	var length uint32
	var err error
	rd := bufio.NewReader(c.conn)
	for {
		if err = binary.Read(rd, binary.LittleEndian, &length); err != nil {
			return
		}
		buf := make([]byte, length)
		if err = binary.Read(rd, binary.LittleEndian, buf); err != nil {
			return
		}
		msg, err := NewMsgPackFromBytes(buf, c.codec)
		if err != nil {
			continue
		}
		go c.do(msg)
	}
}
func (c *Client) Status() bool {
	return c.online.Load()
}
func (c *Client) do(msg *Message) {
	value, ok := c.wait.Load(msg.Sequence)
	if ok {
		ch, ok := value.(chan *Message)
		if !ok {
			return
		}
		ch <- msg
		return
	}
	if c.handle != nil {
		c.handle.Do(c, msg)
	}
}
func (c *Client) Call(method string, request interface{}, timeout time.Duration) (response *Message, err error) {
	if !c.online.Load() {
		return nil, ErrOffline
	}
	seq := c.iter()
	msg := Message{
		Method:   method,
		Sequence: seq,
		codec:    c.codec,
	}
	if err := msg.SetData(request); err != nil {
		return nil, err
	}
	if err := c.Send(&msg); err != nil {
		return nil, err
	}

	ch := make(chan *Message)
	defer close(ch)
	c.wait.Store(seq, ch)
	defer c.wait.Delete(seq)

	if timeout == 0 {
		select {
		case g := <-ch:
			return g, nil
		}
	} else {
		ticker := time.NewTimer(timeout)
		defer ticker.Stop()
		select {
		case g := <-ch:
			return g, nil
		case <-ticker.C:
			return nil, ErrTimeout
		}
	}
}
func (c *Client) CallAsync(method string, request interface{}) (*AsyncResponse, error) {
	if !c.online.Load() {
		return nil, ErrOffline
	}
	seq := c.iter()
	msg := Message{
		Method:   method,
		Sequence: seq,
	}
	if err := msg.SetData(request); err != nil {
		return nil, err
	}
	if err := c.Send(&msg); err != nil {
		return nil, err
	}

	ch := make(chan Message)
	c.wait.Store(seq, ch)
	return &AsyncResponse{
		Ch:     ch,
		Seq:    seq,
		Client: c,
	}, nil
}
func (c *Client) SetExtra(v interface{}) {
	c.extra = v
}
func (c *Client) Extra() interface{} {
	return c.extra
}
func (c *Client) Close() {
	c.running = false
	if c.conn != nil {
		c.lock.Lock()
		defer c.lock.Unlock()
		c.conn.Close()
	}
	c.wait.Range(func(key, value any) bool {
		c.wait.Delete(key)
		return true
	})
}
func (c *Client) SetHandle(h *Handles) {
	c.handle = h
}
func (c *Client) OnConnect(f func(c *Client)) {
	c.onConnect = f
}
func (c *Client) OnDisconnect(f func(c *Client)) {
	c.onDisconnect = f
}
