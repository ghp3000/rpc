package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/ghp3000/nbio"
	nbtls "github.com/ghp3000/nbio/extension/tls"
	"github.com/ghp3000/nbio/logging"
	"github.com/ghp3000/rpc"
	"github.com/ghp3000/validator"
	"github.com/lesismal/llib/std/crypto/tls"
	"log"
	"time"
)

type Session struct {
	*rpc.Client
	bytes.Buffer
}

func main() {
	validator.SetLanguage(validator.LangZh)
	handler := rpc.NewHandles()
	handler.Register("login", func(c *rpc.Client, msg *rpc.Message) (response *rpc.Message) {
		var v struct {
			User     string `msgpack:"user" validate:"required,min=10,max=128"`
			Password string `msgpack:"password" validate:"required,min=5,max=128"`
		}
		fmt.Println(msg.UnmarshalData(&v), v)
		if err := validator.Struct(&v); err != nil {
			fmt.Println(err.Error())
		}
		msg.SetData(map[string]interface{}{"Say": "ok"})
		msg.SetError(rpc.ErrCodeStandard, "用户不存在")
		time.Sleep(time.Millisecond * 1)
		return msg
	})
	handler.Register("hello", func(c *rpc.Client, msg *rpc.Message) (response *rpc.Message) {
		var v string
		fmt.Println(msg.UnmarshalData(&v), v)
		msg.SetData(map[string]interface{}{"Say": "ok"})
		msg.SetError(rpc.ErrCodeStandard, "用户不存在")
		time.Sleep(time.Millisecond * 1)
		return msg
	})
	cert, err := tls.X509KeyPair(rsaCertPEM, rsaKeyPEM)
	if err != nil {
		fmt.Printf("tls.X509KeyPair failed: %v", err)
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}
	logging.SetLevel(0)
	g := nbio.NewEngine(nbio.Config{
		Network: "tcp",
		Addrs:   []string{"localhost:8888"},
	})

	isClient := false
	g.OnOpen(nbtls.WrapOpen(tlsConfig, isClient, func(c *nbio.Conn, tlsConn *tls.Conn) {
		client := rpc.NewClient(tlsConn, &rpc.MsgPackCodec{})
		session := &Session{
			Client: client,
		}
		c.SetExtra(session)
		log.Println("OnOpen:", c.RemoteAddr().String())
	}))
	g.OnClose(nbtls.WrapClose(func(c *nbio.Conn, tlsConn *tls.Conn, err error) {
		log.Println("OnClose:", c.RemoteAddr().String())
	}))

	g.OnData(nbtls.WrapData(func(c *nbio.Conn, tlsConn *tls.Conn, data []byte) {
		iExtra := c.Extra()
		if iExtra == nil {
			c.Close()
			return
		}
		extra, ok := iExtra.(*Session)
		if !ok {
			c.Close()
			return
		}
		extra.Write(data)
		if extra.Len() >= 4 {
			headBuf := extra.Bytes()[:4]
			var length uint32
			if err = binary.Read(bytes.NewReader(headBuf), binary.LittleEndian, &length); err != nil {
				return
			}
			total := 4 + int(length)
			if extra.Len() >= total {
				fmt.Println(extra.Len(), string(extra.Bytes()))
				binary.Read(extra, binary.LittleEndian, &length)
				buf := make([]byte, length)
				binary.Read(extra, binary.LittleEndian, buf)
				msg, err := rpc.NewMsgPackFromBytes(buf, &rpc.MsgPackCodec{})
				if err != nil {
					fmt.Println(err.Error())
					return
				}
				handler.Do(extra.Client, msg)
			}
		}
	}))

	err = g.Start()
	if err != nil {
		fmt.Printf("nbio.Start failed: %v\n", err)
		return
	}
	defer g.Stop()
	g.Wait()
}

var rsaCertPEM = []byte(`-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUJeohtgk8nnt8ofratXJg7kUJsI4wDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMDEyMDcwODIyNThaFw0zMDEy
MDUwODIyNThaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEw
HwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQCy+ZrIvwwiZv4bPmvKx/637ltZLwfgh3ouiEaTchGu
IQltthkqINHxFBqqJg44TUGHWthlrq6moQuKnWNjIsEc6wSD1df43NWBLgdxbPP0
x4tAH9pIJU7TQqbznjDBhzRbUjVXBIcn7bNknY2+5t784pPF9H1v7h8GqTWpNH9l
cz/v+snoqm9HC+qlsFLa4A3X9l5v05F1uoBfUALlP6bWyjHAfctpiJkoB9Yw1TJa
gpq7E50kfttwfKNkkAZIbib10HugkMoQJAs2EsGkje98druIl8IXmuvBIF6nZHuM
lt3UIZjS9RwPPLXhRHt1P0mR7BoBcOjiHgtSEs7Wk+j7AgMBAAGjUzBRMB0GA1Ud
DgQWBBQdheJv73XSOhgMQtkwdYPnfO02+TAfBgNVHSMEGDAWgBQdheJv73XSOhgM
QtkwdYPnfO02+TAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBf
SKVNMdmBpD9m53kCrguo9iKQqmhnI0WLkpdWszc/vBgtpOE5ENOfHGAufHZve871
2fzTXrgR0TF6UZWsQOqCm5Oh3URsCdXWewVMKgJ3DCii6QJ0MnhSFt6+xZE9C6Hi
WhcywgdR8t/JXKDam6miohW8Rum/IZo5HK9Jz/R9icKDGumcqoaPj/ONvY4EUwgB
irKKB7YgFogBmCtgi30beLVkXgk0GEcAf19lHHtX2Pv/lh3m34li1C9eBm1ca3kk
M2tcQtm1G89NROEjcG92cg+GX3GiWIjbI0jD1wnVy2LCOXMgOVbKfGfVKISFt0b1
DNn00G8C6ttLoGU2snyk
-----END CERTIFICATE-----
`)

var rsaKeyPEM = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAsvmayL8MImb+Gz5rysf+t+5bWS8H4Id6LohGk3IRriEJbbYZ
KiDR8RQaqiYOOE1Bh1rYZa6upqELip1jYyLBHOsEg9XX+NzVgS4HcWzz9MeLQB/a
SCVO00Km854wwYc0W1I1VwSHJ+2zZJ2Nvube/OKTxfR9b+4fBqk1qTR/ZXM/7/rJ
6KpvRwvqpbBS2uAN1/Zeb9ORdbqAX1AC5T+m1soxwH3LaYiZKAfWMNUyWoKauxOd
JH7bcHyjZJAGSG4m9dB7oJDKECQLNhLBpI3vfHa7iJfCF5rrwSBep2R7jJbd1CGY
0vUcDzy14UR7dT9JkewaAXDo4h4LUhLO1pPo+wIDAQABAoIBAF6yWwekrlL1k7Xu
jTI6J7hCUesaS1yt0iQUzuLtFBXCPS7jjuUPgIXCUWl9wUBhAC8SDjWe+6IGzAiH
xjKKDQuz/iuTVjbDAeTb6exF7b6yZieDswdBVjfJqHR2Wu3LEBTRpo9oQesKhkTS
aFF97rZ3XCD9f/FdWOU5Wr8wm8edFK0zGsZ2N6r57yf1N6ocKlGBLBZ0v1Sc5ShV
1PVAxeephQvwL5DrOgkArnuAzwRXwJQG78L0aldWY2q6xABQZQb5+ml7H/kyytef
i+uGo3jHKepVALHmdpCGr9Yv+yCElup+ekv6cPy8qcmMBqGMISL1i1FEONxLcKWp
GEJi6QECgYEA3ZPGMdUm3f2spdHn3C+/+xskQpz6efiPYpnqFys2TZD7j5OOnpcP
ftNokA5oEgETg9ExJQ8aOCykseDc/abHerYyGw6SQxmDbyBLmkZmp9O3iMv2N8Pb
Nrn9kQKSr6LXZ3gXzlrDvvRoYUlfWuLSxF4b4PYifkA5AfsdiKkj+5sCgYEAzseF
XDTRKHHJnzxZDDdHQcwA0G9agsNj64BGUEjsAGmDiDyqOZnIjDLRt0O2X3oiIE5S
TXySSEiIkxjfErVJMumLaIwqVvlS4pYKdQo1dkM7Jbt8wKRQdleRXOPPN7msoEUk
Ta9ZsftHVUknPqblz9Uthb5h+sRaxIaE1llqDiECgYATS4oHzuL6k9uT+Qpyzymt
qThoIJljQ7TgxjxvVhD9gjGV2CikQM1Vov1JBigj4Toc0XuxGXaUC7cv0kAMSpi2
Y+VLG+K6ux8J70sGHTlVRgeGfxRq2MBfLKUbGplBeDG/zeJs0tSW7VullSkblgL6
nKNa3LQ2QEt2k7KHswryHwKBgENDxk8bY1q7wTHKiNEffk+aFD25q4DUHMH0JWti
fVsY98+upFU+gG2S7oOmREJE0aser0lDl7Zp2fu34IEOdfRY4p+s0O0gB+Vrl5VB
L+j7r9bzaX6lNQN6MvA7ryHahZxRQaD/xLbQHgFRXbHUyvdTyo4yQ1821qwNclLk
HUrhAoGAUtjR3nPFR4TEHlpTSQQovS8QtGTnOi7s7EzzdPWmjHPATrdLhMA0ezPj
Mr+u5TRncZBIzAZtButlh1AHnpN/qO3P0c0Rbdep3XBc/82JWO8qdb5QvAkxga3X
BpA7MNLxiqss+rCbwf3NbWxEMiDQ2zRwVoafVFys7tjmv6t2Xck=
-----END RSA PRIVATE KEY-----
`)
