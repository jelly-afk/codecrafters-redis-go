package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var _ = net.Listen
var _ = os.Exit

func main() {

	
    l, err := net.Listen("tcp", "0.0.0.0:6379")
    if err != nil {
        os.Exit(1)
    }
    for {
    tcpConn, err := l.Accept()
        if err != nil {
        }
        go handleClient(tcpConn)
    
    }

}

func handleClient (client net.Conn) {
    defer client.Close()
        redisMap := make(map[string][]interface{})
    for {
        readBuffer := make([]byte, 512)
        _, err := client.Read(readBuffer)
        if err != nil {
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
        res, err := handleCommands(parsedResp, redisMap)
        if err != nil {
            log.Fatal(err)
        }
        _, err = client.Write([]byte(res))
        if err != nil {
            return 
        }
    }
}

func parseResp(resp *RESP ) (interface{}, error) {
    switch resp.respStr[resp.idx] {
    case '*':
        endIdx := strings.Index(resp.respStr[resp.idx:], "\r\n")
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
        }
        return res, nil
    case '$':
        endIdx := strings.Index(resp.respStr[resp.idx:], "\r\n")
        strLen, err := strconv.Atoi(resp.respStr[resp.idx+1:resp.idx+endIdx])
        if err != nil {
            return nil, err
        }
        str :=  resp.respStr[resp.idx+endIdx+2:resp.idx+endIdx+2+strLen]
        resp.idx += endIdx+4+strLen
        return str, nil

    }

    return nil, errors.New("invalid resp string")
}

func handleCommands (commands interface{}, rMap map[string][]interface{}) (string, error) {
    c, ok := commands.(string)
    strArr := make([]string, 0)
    if !ok {
        cmds, ok := commands.([]interface{})
        if !ok {
            return "", errors.New("not an array")
        }
        err := errors.New("test")
        strArr, err= interfaceToString(cmds)
        c = strArr[0]
        if err != nil {
            return "", err
        }
    }
    switch c {
    case "PING":
        return "+PONG\r\n", nil
    case "ECHO":
        st := strArr[1]
        return fmt.Sprintf("$%d\r\n%s\r\n",len(st),st), nil
    case "SET":
        var valArr []interface{}
        valArr = append(valArr, strArr[2])
        if len(strArr) > 3 && strArr[3] == "px"{
            exp, err := strconv.Atoi(strArr[4])
            if err != nil {
                return "", err
            }
            valArr = append(valArr,  exp)
            valArr = append(valArr,  time.Now().UnixMilli())
        }
            rMap[strArr[1]] = valArr
        return "+OK\r\n", nil 
    case "GET":
        valArr, ok := rMap[strArr[1]]
        if !ok {
            return "$-1\r\n", nil
        }
        val := valArr[0].(string)
        if len(valArr) > 1{
            exp := valArr[1].(int)
            setTime := valArr[2].(int64)
            if setTime+int64(exp) < time.Now().UnixMilli() {
                return "$-1\r\n", nil
            }
        }
        return fmt.Sprintf("$%d\r\n%s\r\n", len(val), val), nil
    }
    return "", errors.New("invalid commands")

}

func interfaceToString(interfArr []interface{}) ([]string, error) {
    strArr := make([]string, 0)
    for _, s := range interfArr {
        st, ok := s.(string)
        if !ok {
            return nil, errors.New("non string value found")
        }
        trimmedSt := strings.TrimSpace(st)
        if len(trimmedSt) < 1 {
            continue
        }
        strArr = append(strArr, trimmedSt)
    }
    return strArr, nil
}




type RESP struct{
    respStr string
    idx int
}




