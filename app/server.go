package main

import (
	"errors"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/codecrafters-io/redis-starter-go/app/resp"
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

func handleClient(client net.Conn) {
	defer client.Close()
	store := make(map[string]redisValue)
	for {
		readBuffer := make([]byte, 512)
		_, err := client.Read(readBuffer)
		if err != nil {
			return
		}
		resp := &RESP{
			respStr: string(readBuffer),
			idx:     0,
		}
		parsedResp, err := parseResp(resp)
		if err != nil {
			log.Fatal(err)
		}
		res, err := handleCommands(parsedResp, store)
		if err != nil {
			log.Fatal(err)
		}
		_, err = client.Write([]byte(res))
		if err != nil {
			return
		}
	}
}

func parseResp(resp *RESP) (interface{}, error) {
	switch resp.respStr[resp.idx] {
	case '*':
		endIdx := strings.Index(resp.respStr[resp.idx:], "\r\n")
		arrLen, err := strconv.Atoi(resp.respStr[resp.idx+1 : resp.idx+endIdx])
		if err != nil {
			return nil, err
		}
		res := make([]interface{}, arrLen)
		resp.idx += endIdx + 2
		for i := 0; i < arrLen; i++ {
			res[i], err = parseResp(resp)
			if err != nil {
				log.Fatal(err)
			}
		}
		return res, nil
	case '$':
		endIdx := strings.Index(resp.respStr[resp.idx:], "\r\n")
		strLen, err := strconv.Atoi(resp.respStr[resp.idx+1 : resp.idx+endIdx])
		if err != nil {
			return nil, err
		}
		str := resp.respStr[resp.idx+endIdx+2 : resp.idx+endIdx+2+strLen]
		resp.idx += endIdx + 4 + strLen
		return str, nil

	}

	return nil, errors.New("invalid resp string")
}

func handleCommands(commands interface{}, rMap map[string]redisValue) (string, error) {
	c, ok := commands.(string)
	strArr := make([]string, 0)
	if !ok {
		cmds, ok := commands.([]interface{})
		if !ok {
			return "", errors.New("not an array")
		}
		err := errors.New("test")
		strArr, err = interfaceToString(cmds)
		c = strArr[0]
		if err != nil {
			return "", err
		}
	}
	switch c {
	case "PING":
		return resp.PONG, nil
	case "ECHO":
		st := strArr[1]
		return resp.EncodeBulkString(st), nil
	case "SET":
		key := strArr[1]
		val := strArr[2]
		var expiresAt int64
		if len(strArr) > 3 && strArr[3] == "PX" {
			exp, err := strconv.Atoi(strArr[4])
			if err != nil {
				return "", err
			}
			expiresAt = time.Now().UnixMilli() + int64(exp)
		}
		rMap[key] = redisValue{
			value:     val,
			expiresAt: expiresAt,
		}
		return resp.OK, nil
	case "GET":
		redisVal, ok := rMap[strArr[1]]
		if !ok {
			return resp.NULL, nil
		}
		if redisVal.expiresAt > 0 {
			if redisVal.expiresAt < time.Now().UnixMilli() {
				return resp.NULL, nil
			}
		}
		return resp.EncodeBulkString(redisVal.value), nil
	case "CONFIG":
		if len(strArr) < 3 {
			return "", errors.New("invalid commands")
		}
		if strArr[1] == "GET" {
			switch strArr[2] {
			case "dir":
				v, err := getArgs("dir")
				if err != nil {
					return "", err
				}
				return resp.EncodeArray([]any{"dir", v}), nil
			case "dbFilename":
				v, err := getArgs("dbFilename")
				if err != nil {
					return "", err
				}
				return resp.EncodeArray([]any{"dbFilename", v}), nil
			}
		}
	case "RPUSH":
		return resp.EncodeInt(1), nil
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

func getArgs(arg string) (string, error) {
	if len(strings.Trim(arg, " ")) < 1 {
		return "", errors.New("invalid argument")
	}
	for i, s := range os.Args {
		if strings.Trim(s, " ") == "--"+strings.Trim(arg, " ") && len(os.Args) > i+1 {
			return os.Args[i+1], nil
		}
	}
	return "", errors.New("argument not found")
}

type RESP struct {
	respStr string
	idx     int
}

type redisValue struct {
	value     string
	expiresAt int64
}
