package main

import (
	"fmt"
	"github.com/ghp3000/nbio/extension/tls"
	"github.com/ghp3000/rpc"
	"net"
	"time"
)

func dialer() (net.Conn, error) {
	return tls.Dial("tcp4", "localhost:8888", &tls.Config{InsecureSkipVerify: true})
}
func main() {
	cli := rpc.NewClientWithDialer(dialer, true, time.Second*3, &rpc.MsgPackCodec{})
	cli.OnConnect(func(c *rpc.Client) {
		var v struct {
			User     string `msgpack:"user" validate:"required"`
			Password string `msgpack:"password" validate:"required,min=7"`
		}
		v.User = "admin"
		v.Password = "123456"
		msg, err := cli.Call("login", &v, time.Second*3)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println(msg)
		}
	})
	fmt.Println(cli.Status())
	time.Sleep(time.Second)
	for i := 0; i < 10; i++ {
		if msg, err := cli.Call("hello", "xxxxxxxx", time.Second); err != nil {
			fmt.Println("call", err)
		} else {
			fmt.Println(msg.Error())
			fmt.Println(msg, msg.Code, msg.T, msg.Msg)
		}
		time.Sleep(time.Second)
	}
	<-make(chan struct{})
}
