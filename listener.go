package server

import (
	"context"
	"log"
	"net"
	"os/signal"
	"sync/atomic"
	"syscall"
)

type Bss func(uint32) []byte

func CfgBfr() Bss {
	var bs [65536]byte
	var i atomic.Uint32
	return func(l uint32) []byte {
		bgn, end := uint32(0), uint32(0)
	l1:
		end = i.Add(l)
		bgn = end - l
		if end < bgn {
			goto l1
		}
		return bs[bgn:end]
	}
}
func accept(listener net.Listener, cb func(net.Conn)) {
	conn, err := listener.Accept()
	go accept(listener, cb)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer func() {
		conn.Close()
		println("close connection")
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	println("new connection")
	cb(conn)
}
func Listen(network, addr string, cb func(net.Conn)) {
	listener, err := net.Listen(network, addr)
	if err != nil {
		log.Fatalln(err)
	}
	defer listener.Close()
	go accept(listener, cb)
	println("running")
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	<-ctx.Done()
	stop()
}
