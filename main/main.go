package main

import (
	"bytes"
	"log"
	"net"
	"sync"

	"github.com/birowo/server"
)

const connsLen = 9999

var conns = struct {
	conns map[net.Conn]struct{}
	sync.RWMutex
}{make(map[net.Conn]struct{}, connsLen), sync.RWMutex{}}

func chatHandler(conn net.Conn, bfr []byte, hdrs ...[]byte) {
	server.Upgrade(conn, hdrs[0])
	defer func() {
		conns.Lock()
		delete(conns.conns, conn)
		conns.Unlock()
	}()
	conns.Lock()
	conns.conns[conn] = struct{}{}
	conns.Unlock()
	n := int64(0)
	for {
		n_, err := conn.Read(bfr[n:])
		if err != nil {
			log.Println("read:", err)
			return
		}
		n += int64(n_)
		_, opCode, reply, _, l := server.ParseFrame(bfr, n)
		if opCode == server.OpClose || reply == nil {
			return
		}
		if opCode == server.OpPing {
			server.Pong(bfr)
			conn.Write(reply)
		} else {
			conns.RLock()
			println("conns len:", len(conns.conns))
			for conn_ := range conns.conns {
				conns.RUnlock()
				_, err := conn_.Write(reply)
				if err != nil {
					log.Println("write:", err)
				}
				conns.RLock()
			}
			conns.RUnlock()
		}
		n = int64(copy(bfr, bfr[l:n])) //next frame alignment
	}
}

func main() {
	rootHtmPool := server.FilePool("root.htm")
	network := "tcp"
	port := ":8080"
	bssLen := uint32(15)
	bss := server.CfgBfr(bssLen)
	server.Listen(
		network, port,
		func(conn net.Conn) {
			const bsLen uint32 = 1024
			bfr := bss(bsLen)
			n, err := conn.Read(bfr)
			if err != nil {
				log.Println(err.Error())
				return
			}
			mthdEnd := server.Method(bfr, n)
			pathBgn := mthdEnd + 1
			pathEnd, qryBgn, qryEnd := server.PathQuery(bfr, pathBgn, n)
			verBgn := qryEnd + 1
			verEnd := server.Ver(bfr, verBgn, n)
			ifModifiedSince := server.H{"if-modified-since", nil}
			secWebSocketKey := server.H{"sec-websocket-key", nil}
			if server.Headers(bfr, verEnd+2, n, &ifModifiedSince, &secWebSocketKey) > n {
				return
			}
			println(
				"method:", string(bfr[:mthdEnd]),
				",url-path:", string(bfr[pathBgn:pathEnd]),
				",url-query:", string(bfr[qryBgn:qryEnd]),
				",http-version:", string(bfr[verBgn:verEnd]),
			)
			conn.Write(bfr[verBgn:verEnd])
			if bytes.Equal(bfr[:pathEnd], []byte("GET /")) {
				server.SendFile(conn, rootHtmPool, ifModifiedSince.V)
				return
			}
			if bytes.Equal(bfr[:pathEnd], []byte("GET /chat")) {
				chatHandler(conn, bfr, secWebSocketKey.V)
				return
			}
		})
}
