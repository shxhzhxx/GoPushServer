package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

var clients = make(map[int]net.Conn)

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
		conn.SetKeepAlive(true)
		conn.SetKeepAlivePeriod(60 * 1000000000)
		go handleConnection(conn)
	}
}

func handleConnection(conn *net.TCPConn) {
	bufferBytes, err := bufio.NewReader(conn).ReadBytes('\n')

	if err != nil {
		log.Println("client left..")
		conn.Close()
		return
	}

	message := string(bufferBytes)
	clientAddr := conn.RemoteAddr().String()
	response := fmt.Sprintf(message + " from " + clientAddr + "\n")

	log.Println(response)

	conn.Write([]byte("you sent: " + response))

	handleConnection(conn)
}
