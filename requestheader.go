package server

import (
	"bytes"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/birowo/pool"
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

func Path(buf []byte, pathBgn, n int) (pathEnd int, isQuery bool) {
	pathEnd = pathBgn
	for pathEnd < n && buf[pathEnd] != ' ' && buf[pathEnd] != '?' {
		pathEnd++
	}
	isQuery = pathEnd < n && buf[pathEnd] == '?'
	return
}

func Query(buf []byte, queryBgn, n int) (queryEnd int) {
	queryEnd = queryBgn
	for queryEnd < n && buf[queryEnd] != ' ' {
		queryEnd++
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

type V struct {
	Bgn, End int
}
type H struct {
	K string
	V
}

func Headers(buf []byte, i, n int, hs ...*H) int {
	hsLen := len(hs)
	for i < n && buf[i] != '\r' {
		kBgn := i
		for i < n && buf[i] != ':' {
			if buf[i] > ('A'-1) && buf[i] < ('Z'+1) {
				buf[i] |= 0b100000
			}
			i++
		}
		kEnd := i
		i += 2
		j := 0
		for j < hsLen && hs[j].K != string(buf[kBgn:kEnd]) {
			j++
		}
		if j < hsLen {
			hs[j].Bgn = i
			for i < n && buf[i] != '\r' {
				i++
			}
			hs[j].End = i
			i += 2
			hsLen--
			println(
				"K:", hs[j].K,
				"V:", string(
					buf[hs[j].Bgn:hs[j].End],
				),
			)
			hs[j], hs[hsLen] = hs[hsLen], hs[j]
		} else {
			for i < n && buf[i] != '\r' {
				i++
			}
			i += 2
		}
	}
	return i
}

var (
	OK          = []byte(" 200 OK\r\n")
	NotModified = []byte(" 304 Not Modified\r\n\r\n")
)

var Date atomic.Pointer[[]byte]

func init() {
	go func() {
		const (
			dt  = "Date: "
			dtL = len(dt)
			hTF = http.TimeFormat
		)
		date := []byte(dt + hTF + "\r\n\r\n")
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

func FilePool(name string) *pool.Pool[F] {
	ct := Mime[name[strings.LastIndexByte(name, '.'):]]
	//println("content-type:", string(ct))
	return pool.New[F](
		1024,
		func() F {
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
	)
}
func SendFile(conn net.Conn, filePool *pool.Pool[F], ifModifiedSince []byte) {
	file := filePool.Get()
	if bytes.Equal(ifModifiedSince, file.Mod) {
		conn.Write(NotModified)
		return
	}
	conn.Write(file.Hdrs)
	conn.Write(*Date.Load())
	file.Seek(0, 0)
	_, err := file.WriteTo(conn)
	if err != nil {
		log.Println("send file:", err)
	}
	filePool.Put(file)
}
