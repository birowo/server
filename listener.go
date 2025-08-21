package server

import (
	"context"
	"log"
	"net"
	"os/signal"
	"sync"
	"syscall"
)

func accept(listener net.Listener, bufPool *sync.Pool, cb func(net.Conn, []byte)) {
	conn, err := listener.Accept()
	go accept(listener, bufPool, cb)
	if err != nil {
		log.Println(err.Error())
		return
	}
	buf := bufPool.Get()
	println("new connection")
	defer func() {
		bufPool.Put(buf)
		conn.Close()
		println("close connection")
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	cb(conn, buf.([]byte))
}
func Listen(network, addr string, bufLen int, cb func(net.Conn, []byte)) {
	listener, err := net.Listen(network, addr)
	if err != nil {
		log.Fatalln(err)
	}
	defer listener.Close()
	bufPool := &sync.Pool{
		New: func() any {
			return make([]byte, bufLen)
		},
	}
	go accept(listener, bufPool, cb)
	println("running")
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	<-ctx.Done()
	stop()
}
