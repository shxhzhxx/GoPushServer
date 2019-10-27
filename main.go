package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

const BUFF_SIZE = 512

var clients = make(map[int]*net.TCPConn)

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

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func handleConnection(conn *net.TCPConn) {
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(60 * 1000000000)
	defer conn.Close()

	// var id uint
	buffer := make([]byte, BUFF_SIZE)
	for {
		n, err := conn.Read(buffer[0:1])
		if err != nil || n != 1 {
			log.Println("broken link")
			return
		}
		switch cmd := buffer[0]; cmd {
		case 0: //echo
			for pos := 0; pos < 4; {
				n, err := conn.Read(buffer[pos:4])
				if err != nil {
					log.Println("broken link")
					return
				}
				pos += n
			}
			len := int(binary.BigEndian.Uint32(buffer[0:4]))
			conn.Write(buffer[0:4])
			for len > 0 {
				n, err := conn.Read(buffer[0:min(BUFF_SIZE, len)])
				if err != nil {
					log.Println("broken link")
					return
				}
				conn.Write(buffer[0:n])
				len -= n
			}
		case 1: //bind
			conn.Write([]byte("bind"))
		default:
			conn.Write([]byte(fmt.Sprintf("\nunknow cmd: %v\n", cmd)))
			log.Println("unknow cmd: ", cmd)
			return
		}

		// pos := 0
		// for pos < 4 {
		// 	n, err := conn.Read(buffer[pos:4])
		// 	if err != nil {
		// 		log.Println("broken link")
		// 		return
		// 	}
		// 	pos += n
		// }
		// len := int(binary.BigEndian.Uint32(buffer[0:4]))
		// if len < 5 {
		// 	conn.Write([]byte(fmt.Sprintf("invalid len %v", len)))
		// 	log.Println("invalid len:", len)
		// 	return
		// }

		// log.Println("len:", len, buffer)
		// for pos < len {
		// 	n, err := conn.Read(buffer[pos:len])
		// 	if err != nil {
		// 		log.Println("broken link")
		// 		return
		// 	}
		// 	pos += n
		// }
		// log.Println("len:", len, buffer)
	}
}
