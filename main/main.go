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
	bss := server.CfgBfr()
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
			println(string(bfr[:n]))
			mthdEnd := server.Method(bfr, n)
			println(string(bfr[:mthdEnd]))
			pathBgn := mthdEnd + 1
			pathEnd, isQuery := server.Path(bfr, pathBgn, n)
			println(string(bfr[:pathEnd]))
			var qryBgn, qryEnd, verBgn int
			if isQuery {
				qryBgn = pathEnd + 1
				qryEnd = server.Query(bfr, qryBgn, n)
				verBgn = qryEnd + 1
			} else {
				verBgn = pathEnd + 1
			}
			verEnd := server.Ver(bfr, verBgn, n)
			println(string(bfr[verBgn:verEnd]))
			ifModifiedSince := server.H{K: "if-modified-since"}
			secWebSocketKey := server.H{K: "sec-websocket-key"}
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
				imsV := ifModifiedSince.V
				server.SendFile(conn, rootHtmPool, bfr[imsV.Bgn:imsV.End])
				return
			}
			if bytes.Equal(bfr[:pathEnd], []byte("GET /chat")) {
				swkV := secWebSocketKey.V
				chatHandler(conn, bfr, bfr[swkV.Bgn:swkV.End])
				return
			}
		})
}
