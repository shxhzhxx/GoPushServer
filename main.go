package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

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

func handleConnection(conn *net.TCPConn) {
	defer conn.Close()
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(60 * 1000000000)
	buffer := make([]byte, 1024)
	channel := make(chan byte, 128)
	for {
		pos := 0
		for pos < 4 {
			n, err := conn.Read(buffer[pos:4])
			if err != nil {
				log.Println("broken link")
				return
			}
			pos += n
		}
		len := int(binary.BigEndian.Uint32(buffer[0:4]))
		if len < 5 || len > 1024 {
			conn.Write([]byte(fmt.Sprintf("invalid len %v", len)))
			log.Println("invalid len:", len)
			return
		}
		log.Println("len:", len, buffer)
		for pos < len {
			n, err := conn.Read(buffer[pos:len])
			if err != nil {
				log.Println("broken link")
				return
			}
			pos += n
		}
		log.Println("len:", len, buffer)
	}
	// bufferBytes, err := bufio.NewReader(conn).ReadBytes('\n')

	// if err != nil {
	// 	log.Println("client left..")

	// 	conn.Close()
	// 	return
	// }

	// message := string(bufferBytes)
	// clientAddr := conn.RemoteAddr().String()
	// response := fmt.Sprintf(message + " from " + clientAddr + "\n")

	// log.Println(response)

	// conn.Write([]byte("you sent: " + response))

	// handleConnection(conn)
}
