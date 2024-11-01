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
    tcpConn, err := l.Accept()
    defer tcpConn.Close()
    if err != nil {
        fmt.Println("Error accepting connection: ", err.Error())
        os.Exit(1)
    }
    for {
        readBuffer := make([]byte, 512)
        _, err = tcpConn.Read(readBuffer)
        if err != nil {
            log.Fatal(err)
        }
        fmt.Println(bytes.Compare(readBuffer, []byte("*1\r\n$4\r\nPING\r\n")))
        _, err = tcpConn.Write([]byte("+PONG\r\n"))
        if err != nil {
            log.Fatal(err)
        }
        fmt.Println("readBuffer: ", string(readBuffer))

    }

    

}









