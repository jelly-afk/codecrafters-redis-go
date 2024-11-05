package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
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
        resp := &RESP{
            respStr: string(readBuffer),
            idx: 0,
        }
        parsedResp, err := parseResp(resp)
        if err != nil {
            log.Fatal(err)
        }
        fmt.Println("pREsp: ", parsedResp)
        res, err := handleCommands(parsedResp)
        fmt.Println("res: ", res)
        if err != nil {
            log.Fatal(err)
        }
        _, err = client.Write([]byte(res))
        if err != nil {
            log.Println("error writing to a client: ", err)
            return 
        }
        fmt.Println("readBuffer: ", string(readBuffer))
    }
}

func parseResp(resp *RESP ) (interface{}, error) {
    switch resp.respStr[resp.idx] {
    case '*':
        endIdx := strings.Index(resp.respStr[resp.idx:], "\r\n")
        fmt.Println("eidx: ", endIdx)
        fmt.Println("resp: ", resp.respStr)
        arrLen, err := strconv.Atoi(resp.respStr[resp.idx+1:resp.idx+endIdx])
        if err != nil {
            return nil, err
        }
        res := make([]interface{}, arrLen)
        resp.idx += endIdx+2
        for i:=0;i<arrLen;i++ {
            res[i], err = parseResp(resp)
            if err != nil {
                log.Fatal(err)
            }
            fmt.Println("resi ", res[i])
        }
        return res, nil
    case '$':
        endIdx := strings.Index(resp.respStr[resp.idx:], "\r\n")
        fmt.Println()
        fmt.Printf("st: %d, end: %d\n", resp.idx, endIdx)
        strLen, err := strconv.Atoi(resp.respStr[resp.idx+1:resp.idx+endIdx])
        if err != nil {
            return nil, err
        }
        str :=  resp.respStr[resp.idx+endIdx+2:resp.idx+endIdx+2+strLen]
        resp.idx += endIdx+4+strLen
        fmt.Println("str: ", str)
        return str, nil

    }

    return nil, errors.New("invalid resp string")
}

func handleCommands (commands interface{}) (string, error) {
    c, ok := commands.(string)
    fmt.Println("cmdss: ", commands)
    
    arr, ok := commands.([]interface{})
    if ok {
        c, ok = arr[0].(string)
        fmt.Printf("cmds: %v", arr)

    }
    fmt.Println("cmd: ", c)
    switch c {
    case "PING":
        return "+PONG\r\n", nil
    case "ECHO":
        st, ok := arr[1].(string)
        if !ok {
            return "", errors.New("invalis commands")
        }
        return fmt.Sprintf("$%d\r\n%s\r\n",len(st),st), nil
    }
    return "", errors.New("invalid commands")

}

type RESP struct{
    respStr string
    idx int
}




