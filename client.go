package rpc

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
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
	onErr        func(err error)
}

func NewClientWithDialer(dialer func() (net.Conn, error), reconnect bool, delay time.Duration) *Client {
	c := &Client{conn: nil, delay: delay, dialer: dialer, reconnect: func(r bool) uint32 {
		if r {
			return 1
		} else {
			return 0
		}
	}(reconnect)}
	return c
}

func NewClient(conn net.Conn) *Client {
	c := &Client{conn: conn, online: atomic.Bool{}}
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
	b.Write(buf)
	buf = b.Bytes()
	if _, err = c.conn.Write(buf); err != nil {
		return err
	}
	//if _, err = c.conn.Write(buf); err != nil {
	//	return err
	//}
	return nil
}
func (c *Client) iter() (seq uint32) {
	return atomic.AddUint32(&c.seq, 1)
}
func (c *Client) loop() {
	var err error
	c.running = true
	for c.running {
		if c.dialer != nil {
			c.conn, err = c.dialer()
			if err != nil {
				if c.onErr != nil {
					c.onErr(fmt.Errorf("dial error: %v", err))
				}
				if atomic.LoadUint32(&c.reconnect) == 0 {
					c.Close()
					return
				}
				time.Sleep(c.delay)
				atomic.AddUint32(&c.reconnect, 1)
				continue
			}
		}
		c.recvLoop()
		if c.onDisconnect != nil {
			c.onDisconnect(c)
		}
		time.Sleep(c.delay)
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
			if c.onErr != nil {
				c.onErr(fmt.Errorf("read length error: %v", err))
			}
			return
		}
		buf := make([]byte, length)
		if err = binary.Read(rd, binary.LittleEndian, buf); err != nil {
			if c.onErr != nil {
				c.onErr(fmt.Errorf("read data error: %v", err))
			}
			return
		}
		msg, err := NewMessageFromBytes(buf)
		if err != nil {
			if c.onErr != nil {
				c.onErr(fmt.Errorf("parse data error: %v", err))
			}
			return
		}
		go c.Do(msg)
	}
}
func (c *Client) Status() bool {
	return c.online.Load()
}
func (c *Client) Do(msg *Message) {
	defer func() {
		if r := recover(); r != nil {
		}
	}()
	if msg.Reply {
		value, ok := c.wait.Load(msg.Sequence)
		if ok {
			ch, ok := value.(chan *Message)
			if !ok {
				return
			}
			ch <- msg
		}
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
		Reply:    false,
		Sequence: seq,
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
	defer func() {
		if r := recover(); r != nil {
		}
	}()
	c.running = false
	c.wait.Range(func(key, value any) bool {
		c.wait.Delete(key)
		return true
	})
	if c.conn != nil {
		c.lock.Lock()
		defer c.lock.Unlock()
		_ = c.conn.Close()
	}
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
func (c *Client) OnError(f func(err error)) {
	c.onErr = f
}
func (c *Client) RemoteAddr() string {
	if c.conn != nil {
		return c.conn.RemoteAddr().String()
	}
	return "0.0.0.0:0"
}
func (c *Client) LocalAddr() string {
	if c.conn != nil {
		return c.conn.LocalAddr().String()
	}
	return "0.0.0.0:0"
}
