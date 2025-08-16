package server

import (
	"crypto/sha1"
	"encoding/base64"
	"log"
	"net"
	"sync"
)

var upgrade *sync.Pool

func init() {
	upgrade = &sync.Pool{
		New: func() any {
			/*const x = len(" 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\n" +
			"Connection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: ")*/
			return []byte(" 101 Switching Protocols\r\n" +
				"Upgrade: websocket\r\n" +
				"Connection: Upgrade\r\n" +
				"Sec-WebSocket-Accept: " +
				"0123456789112345678921234567\r\n\r\n",
			)
		},
	}
}

func Upgrade(conn net.Conn, secWebsocketKey []byte) {
	key := []byte("012345678911234567892123258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
	copy(key, secWebsocketKey)
	hash := sha1.Sum(key)
	buf_ := upgrade.Get()
	buf := buf_.([]byte)
	base64.StdEncoding.Encode(buf[89:], hash[:])
	//println(string(buf))
	_, err := conn.Write(buf)
	if err != nil {
		log.Println("upgrade:", err)
	}
	upgrade.Put(buf_)
}
func unmask(payload []byte, len_ int64, maskKey []byte) {
	for i := range len_ {
		payload[i] ^= maskKey[i&3]
	}
}

const (
	OpCont  = 0x00
	OpTxt   = 0x01
	OpBin   = 0x02
	OpClose = 0x08
	OpPing  = 0x09
	OpPong  = 0x0a
)

func ParseFrame(bfr []byte, n int64) (fin, opCode byte, reply []byte, hdrLen, l int64) {
	// payload form client to server should be masked
	if n < 6 || (bfr[1]&0x80) != 0x80 {
		return //return if not enough data or not masked
	}
	bfr[1] &= 0x7f
	len_ := int64(bfr[1])
	switch len_ {
	case 126:
		len_ = (int64(bfr[2]) << 8) | int64(bfr[3])
		l = 8 + len_
		if n < l {
			return
		}
		hdrLen = 4
	case 127:
		return //not implemented yet
	default:
		l = 6 + len_
		if n < l {
			return
		}
		hdrLen = 2
	}
	z := hdrLen + 4
	unmask(bfr[z:l], len_, bfr[hdrLen:z])
	copy(bfr[hdrLen:], bfr[z:l])
	reply = bfr[:hdrLen+len_]
	fin = bfr[0] & 0x80
	opCode = bfr[0] & 0x0f
	//payload := reply[hdrLen:]
	return
}
func Pong(bfr []byte) {
	bfr[0] ^= 0b11 //pong <- ping
}
