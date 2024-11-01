package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
)

var _ = net.Listen
var _ = os.Exit

func main() {
	fmt.Println("Logs from your program will appear here!")

	
    l, err := net.Listen("tcp", "0.0.0.0:6379")
    if err != nil {
        fmt.Println("Failed to bind to port 6379")
        os.Exit(1)
    }
    for {
    tcpConn, err := l.Accept()
        if err != nil {
            log.Println("error accepting a client: ", err)
        }
        go handleClient(tcpConn)
    
    }

}

func handleClient (client net.Conn) {
    defer client.Close()
    for {
        readBuffer := make([]byte, 512)
        _, err := client.Read(readBuffer)
        if err != nil {
            log.Println("error reading from a client: ", err)
            return
        }
        fmt.Println(bytes.Compare(readBuffer, []byte("*1\r\n$4\r\nPING\r\n")))
        _, err = client.Write([]byte("+PONG\r\n"))
        if err != nil {
            log.Println("error writing to a client: ", err)
            return 
        }
        fmt.Println("readBuffer: ", string(readBuffer))
    }
}









