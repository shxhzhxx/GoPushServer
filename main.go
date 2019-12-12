package main

import (
	"encoding/binary"
	"log"
	"net"
	"sync"
)

const BUFF_SIZE = 32 * 1024

const CMD_ERR = 0
const ERR_UNKNOWN_CMD = 0
const ERR_REPEAT_BINDING = 1
const ERR_BIND_ID_CONFLICT = 2
const ERR_BIND_ID_INVALID = 3

const CMD_ECHO = 1
const CMD_BIND = 2
const CMD_PUSH = 3
const CMD_BROADCAST = 4
const CMD_IP = 5

var conns = make(map[*net.TCPConn]bool)
var clients = make(map[uint32]*net.TCPConn)
var mux = sync.Mutex{}

func bindClients(id uint32, conn *net.TCPConn) bool {
	mux.Lock()
	defer mux.Unlock()
	_, ok := clients[id]
	if !ok {
		clients[id] = conn
	}
	return !ok
}

func main() {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 8080})
	if err != nil {
		log.Fatal("tcp server listener error:", err)
	}
	go test()

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Fatal("tcp server accept error", err)
		}
		go handleConnection(conn)
	}
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func stuffFull(conn *net.TCPConn, buffer []byte) {
	end := len(buffer)
	for pos := 0; pos < end; {
		n, err := conn.Read(buffer[pos:end])
		if err != nil {
			panic(err)
		}
		pos += n
	}
}

func consume(conn *net.TCPConn, buffer []byte, size int, consumer func([]byte)) {
	len := len(buffer)
	for size > 0 {
		n, err := conn.Read(buffer[0:min(len, size)])
		if err != nil {
			panic(err)
		}
		consumer(buffer[0:n])
		size -= n
	}
}

/**
用channel构建buffer池，不需要每个client一个buffer，节省内存。
增加超时机制，开始执行一个命令后，x秒内没有收到更多数据则断开链接，防止客户端错误引起的内存泄漏。（要考虑大文件传输的情况，不能给整个命令一个执行超时）
*/
func handleConnection(conn *net.TCPConn) {
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(10 * 1000000000)

	conns[conn] = true
	var id uint32 = 0
	closeClient := func() {
		log.Println("close client:", id)
		delete(conns, conn)
		conn.Close()
		if id != 0 {
			mux.Lock()
			defer mux.Unlock()
			delete(clients, id)
		}
	}
	defer closeClient()

	errBuffer := []byte{CMD_ERR, 0, 0}
	sendErr := func(err uint16) {
		binary.BigEndian.PutUint16(errBuffer[1:3], err)
		conn.Write(errBuffer)
	}

	intBuffer := make([]byte, 4)
	readInt := func() uint32 {
		stuffFull(conn, intBuffer)
		return binary.BigEndian.Uint32(intBuffer)
	}
	writeIntTo := func(c *net.TCPConn, value uint32) {
		binary.BigEndian.PutUint32(intBuffer, value)
		_, err := c.Write(intBuffer)
		if err != nil {
			panic(err)
		}
	}
	writeInt := func(value uint32) {
		writeIntTo(conn, value)
	}
	readCmd := func() byte {
		_, err := conn.Read(intBuffer[0:1])
		if err != nil {
			panic(err)
		}
		return intBuffer[0]
	}
	writeCmdTo := func(c *net.TCPConn, cmd byte) {
		intBuffer[0] = cmd
		_, err := c.Write(intBuffer[0:1])
		if err != nil {
			panic(err)
		}
	}
	writeCmd := func(cmd byte) {
		writeCmdTo(conn, cmd)
	}

	buffer := make([]byte, BUFF_SIZE)
	read := func(size uint32, consumer func([]byte)) {
		consume(conn, buffer, int(size), consumer)
	}
	for {
		switch cmd := readCmd(); cmd {
		case CMD_ECHO:
			len := readInt()
			writeCmd(cmd)
			writeInt(len)
			read(len, func(data []byte) {
				conn.Write(data)
			})
		case CMD_BIND:
			if id != 0 {
				sendErr(ERR_REPEAT_BINDING)
				return
			}
			id = readInt()
			if id == 0 {
				sendErr(ERR_BIND_ID_INVALID)
				return
			}
			ok := bindClients(id, conn)
			if !ok {
				sendErr(ERR_BIND_ID_CONFLICT)
				return
			}
		case CMD_PUSH:
			ids := make([]uint32, readInt())
			for i := range ids {
				ids[i] = readInt()
			}
			len := readInt()
			for _, id := range ids {
				c, ok := clients[id]
				if ok {
					writeCmdTo(c, CMD_PUSH)
					writeIntTo(c, len)
				}
			}
			read(len, func(data []byte) {
				for _, id := range ids {
					c, ok := clients[id]
					if ok {
						c.Write(data)
					}
				}
			})
		case CMD_BROADCAST:
			len := readInt()
			for c := range conns {
				if c != conn {
					writeCmdTo(c, CMD_BROADCAST)
					writeIntTo(c, len)
				}
			}
			read(len, func(data []byte) {
				for c := range conns {
					if c != conn {
						c.Write(data)
					}
				}
			})
		case CMD_IP:
			addr := []byte(conn.RemoteAddr().String())
			binary.BigEndian.PutUint32(buffer[1:5], uint32(len(addr)))
			conn.Write(buffer[0:5])
			conn.Write(addr)
		default:
			sendErr(ERR_UNKNOWN_CMD)
			return
		}
	}
}

func test() {

}
