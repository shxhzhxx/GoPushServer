package main

import (
	"encoding/binary"
	"log"
	"net"
	"sync"
)

const BUFF_SIZE = 16

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

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Fatal("tcp server accept error", err)
		}
		go handleConnection(conn)
	}
}

func min(x, y uint32) uint32 {
	if x < y {
		return x
	}
	return y
}

func handleConnection(conn *net.TCPConn) {
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(10 * 1000000000)

	conns[conn] = true
	addr := []byte(conn.RemoteAddr().String())
	var id uint32 = 0
	closeClient := func() {
		log.Println("close client:", id)
		if id != 0 {
			delete(clients, id)
		}
		delete(conns, conn)
		conn.Close()
	}
	defer closeClient()

	buffer := make([]byte, BUFF_SIZE)
	sendErr := func(err uint16) {
		buffer[0] = CMD_ERR
		binary.BigEndian.PutUint16(buffer[1:3], err)
		conn.Write(buffer[0:3])
	}
	writeToBuffer := func(start, offset int) bool {
		end := start + offset
		for pos := start; pos < end; {
			n, err := conn.Read(buffer[pos:end])
			if err != nil {
				log.Println("broken link")
				return false
			}
			pos += n
		}
		return true
	}
	read := func(len uint32, consumer func([]byte)) bool {
		for len > 0 {
			n, err := conn.Read(buffer[0:min(BUFF_SIZE, len)])
			if err != nil {
				log.Println("broken link")
				return false
			}
			consumer(buffer[0:n])
			len -= uint32(n)
		}
		return true
	}
	for {
		if !writeToBuffer(0, 1) {
			return
		}
		switch cmd := buffer[0]; cmd {
		case CMD_ECHO:
			if !writeToBuffer(1, 4) {
				return
			}
			len := binary.BigEndian.Uint32(buffer[1:5])
			conn.Write(buffer[0:5])
			if !read(len, func(data []byte) {
				conn.Write(data)
			}) {
				return
			}
		case CMD_BIND:
			if id != 0 {
				sendErr(ERR_REPEAT_BINDING)
				return
			}
			if !writeToBuffer(0, 4) {
				return
			}
			id = binary.BigEndian.Uint32(buffer[0:4])
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
			if !writeToBuffer(0, 4) {
				return
			}
			ids := make([]uint32, binary.BigEndian.Uint32(buffer[0:4]))
			for i := range ids {
				if !writeToBuffer(0, 4) {
					return
				}
				ids[i] = binary.BigEndian.Uint32(buffer[0:4])
			}
			if !writeToBuffer(0, 4) {
				return
			}
			len := binary.BigEndian.Uint32(buffer[0:4])
			if !read(len, func(data []byte) {
				for _, id := range ids {
					c, ok := clients[id]
					if ok {
						c.Write(data)
					}
				}
			}) {
				return
			}
		case CMD_BROADCAST:
			if !writeToBuffer(0, 4) {
				return
			}
			len := binary.BigEndian.Uint32(buffer[0:4])
			if !read(len, func(data []byte) {
				for c := range conns {
					if c != conn {
						c.Write(data)
					}
				}
			}) {
				return
			}
		case CMD_IP:
			binary.BigEndian.PutUint32(buffer[1:5], uint32(len(addr)))
			conn.Write(buffer[0:5])
			conn.Write(addr)
		default:
			sendErr(ERR_UNKNOWN_CMD)
			log.Println("unknown cmd:", cmd)
			return
		}
	}
}
