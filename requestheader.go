package server

import (
	"bytes"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func BytesInt(bs []byte, q int64) {
	i := len(bs)
	for q > 0 {
		i--
		bs[i] = byte(q%10) + '0'
		q /= 10
	}
}
func IntBytes(bs []byte) (q int64) {
	for _, v := range bs {
		if v < '0' || v > '9' {
			return 0
		}
		q *= 10
		q += int64(v - '0')
	}
	return
}
func Method(buf []byte, n int) (mthdEnd int) {
	for mthdEnd < n && buf[mthdEnd] != ' ' {
		mthdEnd++
	}
	return
}
func PathQuery(buf []byte, pathBgn, n int) (pathEnd, qryBgn, qryEnd int) {
	qryEnd = pathBgn
	for qryEnd < n && buf[qryEnd] != ' ' {
		if buf[qryEnd] == '?' {
			pathEnd = qryEnd
			qryEnd++
			qryBgn = qryEnd
		} else {
			qryEnd++
		}
	}
	if qryBgn == 0 {
		pathEnd = qryEnd
	}
	return
}
func Ver(buf []byte, verBgn, n int) (verEnd int) {
	verEnd = verBgn
	for verEnd < n && buf[verEnd] != '\r' {
		verEnd++
	}
	return
}

type H struct {
	K string
	V []byte
}

func kvs(key, val []byte, hsLen int, hs []*H) int {
	l := 0
	for l < hsLen && string(key) != string(hs[l].K) {
		l++
	}
	if l < hsLen {
		hs[l].V = val
		hsLen--
		hs[l], hs[hsLen] = hs[hsLen], hs[l]
	}
	return hsLen
}
func loCase(buf []byte, i int) {
	if buf[i] > ('A'-1) && buf[i] < ('Z'+1) {
		buf[i] |= 0b100000
	}
}
func key(buf []byte, i, n int) int {
	for i < n && buf[i] != ':' {
		loCase(buf, i)
		i++
	}
	return i
}
func val(buf []byte, i, n int) int {
	for i < n && buf[i] != '\r' {
		i++
	}
	return i
}
func Headers(buf []byte, i, n int, hs ...*H) int {
	hsLen := len(hs)
	for i < n && buf[i] != '\r' {
		j := i
		k := key(buf, j, n) + 2
		i = val(buf, k, n) + 2
		hsLen = kvs(buf[j:k-2], buf[k:i-2], hsLen, hs)
	}
	return i
}

var (
	OK          = []byte(" 200 OK\r\n")
	NotModified = []byte(" 304 Not Modified\r\n\r\n")
)

var Date atomic.Pointer[[]byte]

func init() {
	const (
		dt  = "Date: "
		dtL = len(dt)
		hTF = http.TimeFormat
	)
	date := []byte(dt + hTF + "\r\n\r\n")
	go func() {
		for {
			copy(date[dtL:], time.Now().Format(hTF))
			Date.Store(&date)
			time.Sleep(time.Second)
		}
	}()
}

var RN = []byte("\r\n")

func HexInt(q int64) []byte {
	if q < 0 {
		return nil
	}
	if q == 0 {
		return []byte("0")
	}
	var bs [16]byte
	i := 16
	for q > 0 {
		r := byte(q) & 0xf
		i--
		if r < 10 {
			bs[i] = r + '0'
		} else {
			bs[i] = r - 10 + 'a'
		}
		q >>= 4
	}
	return bs[i:]
}

var Mime = map[string][]byte{
	".htm": []byte("text/html; charset=utf-8"),
}

type F struct {
	*os.File
	Hdrs []byte
	Mod  []byte
	Err  error
}

func FilePool(name string) *sync.Pool {
	ct := Mime[name[strings.LastIndexByte(name, '.'):]]
	//println("content-type:", string(ct))
	return &sync.Pool{
		New: func() any {
			f, err := os.Open(name)
			if err != nil {
				return F{nil, nil, nil, err}
			}
			i, err := f.Stat()
			if err != nil {
				return F{nil, nil, nil, err}
			}
			//const x = len(" 200 OK\r\nLast-Modified: Mon, 02 Jan 2006 15:04:05 GMT\r\nContent-Length: 0000000000\r\nContent-Type: ")
			Hdrs := []byte(" 200 OK\r\nLast-Modified: Mon, 02 Jan 2006 15:04:05 GMT\r\nContent-Length: 0000000000\r\nContent-Type:                             \r\n")
			copy(Hdrs[24:], i.ModTime().Format(http.TimeFormat))
			copy(Hdrs[97:], ct)
			BytesInt(Hdrs[:81], i.Size())
			return F{f, Hdrs, Hdrs[24:53], nil}
		},
	}
}
func SendFile(conn net.Conn, filePool *sync.Pool, ifModifiedSince []byte) {
	file_ := filePool.Get()
	file := file_.(F)
	if bytes.Equal(ifModifiedSince, file.Mod) {
		conn.Write(NotModified)
		return
	}
	conn.Write(file.Hdrs)
	conn.Write(*Date.Load())
	file.Seek(0, 0)
	file.WriteTo(conn)
	filePool.Put(file_)
}
