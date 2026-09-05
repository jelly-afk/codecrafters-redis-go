package main

import (
	"errors"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codecrafters-io/redis-starter-go/app/resp"
)

type Store struct {
	data      map[string]redisValue
	list      map[string][]string
	listChans map[string][]chan struct{}
	mu        *sync.Mutex
}

func newStore() *Store {
	return &Store{
		data:      make(map[string]redisValue),
		list:      make(map[string][]string),
		listChans: map[string][]chan struct{}{},
		mu:        &sync.Mutex{},
	}
}

type redisValue struct {
	value     string
	expiresAt int64
}

// RESP holds the raw RESP protocol string and a cursor index for parsing.
type RESP struct {
	respStr string
	idx     int
}

// commandHandler is the function signature all command handlers must satisfy.
type commandHandler func(args []string, s *Store) (string, error)

// handlers maps command names to their handler functions.
var handlers = map[string]commandHandler{
	"PING":   handlePing,
	"ECHO":   handleEcho,
	"SET":    handleSet,
	"GET":    handleGet,
	"CONFIG": handleConfig,
	"RPUSH":  handleRPush,
	"LPUSH":  handleLPush,
	"LRANGE": handleLRange,
	"LLEN":   handleLLen,
	"LPOP":   handleLPop,
	"BLPOP":  handleBLPop,
}

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		os.Exit(1)
	}
	store := newStore()
	for {
		tcpConn, err := l.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleClient(tcpConn, store)
	}
}

func handleClient(client net.Conn, store *Store) {
	defer client.Close()
	for {
		readBuffer := make([]byte, 512)
		n, err := client.Read(readBuffer)
		if err != nil {
			log.Println(err)
			return
		}
		r := &RESP{
			respStr: string(readBuffer[:n]),
			idx:     0,
		}
		parsedResp, err := parseResp(r)
		if err != nil {
			log.Println(err)
			return
		}
		res, err := handleCommands(parsedResp, store)
		if err != nil {
			log.Println(err)
			return
		}
		_, err = client.Write([]byte(res))
		if err != nil {
			return
		}
	}
}

// handleCommands is a function dispatcher
func handleCommands(commands any, s *Store) (string, error) {
	args, err := toStringSlice(commands)
	if err != nil {
		return "", err
	}
	if len(args) == 0 {
		return "", errors.New("empty command")
	}

	cmd := strings.ToUpper(args[0])
	handler, ok := handlers[cmd]
	if !ok {
		return "", errors.New("unknown command: " + cmd)
	}
	return handler(args, s)
}

func toStringSlice(commands interface{}) ([]string, error) {
	switch v := commands.(type) {
	case string:
		return []string{strings.TrimSpace(v)}, nil
	case []interface{}:
		return interfaceToString(v)
	default:
		return nil, errors.New("unsupported command type")
	}
}

func handlePing(_ []string, _ *Store) (string, error) {
	return resp.PONG, nil
}

func handleEcho(args []string, _ *Store) (string, error) {
	if len(args) < 2 {
		return "", errors.New("ECHO requires an argument")
	}
	return resp.EncodeBulkString(args[1]), nil
}

func handleSet(args []string, s *Store) (string, error) {
	if len(args) < 3 {
		return "", errors.New("SET requires key and value")
	}
	key := args[1]
	val := args[2]
	var expiresAt int64
	if len(args) > 3 && strings.ToUpper(args[3]) == "PX" {
		if len(args) < 5 {
			return "", errors.New("SET PX requires a millisecond value")
		}
		exp, err := strconv.Atoi(args[4])
		if err != nil {
			return "", err
		}
		expiresAt = time.Now().UnixMilli() + int64(exp)
	}
	s.data[key] = redisValue{value: val, expiresAt: expiresAt}
	return resp.OK, nil
}

func handleGet(args []string, s *Store) (string, error) {
	if len(args) < 2 {
		return "", errors.New("GET requires a key")
	}
	redisVal, ok := s.data[args[1]]
	if !ok {
		return resp.NULL, nil
	}
	if redisVal.expiresAt > 0 && redisVal.expiresAt < time.Now().UnixMilli() {
		return resp.NULL, nil
	}
	return resp.EncodeBulkString(redisVal.value), nil
}

func handleConfig(args []string, _ *Store) (string, error) {
	if len(args) < 3 {
		return "", errors.New("CONFIG requires a subcommand and parameter")
	}
	if strings.ToUpper(args[1]) != "GET" {
		return "", errors.New("unsupported CONFIG subcommand: " + args[1])
	}
	param := args[2]
	v, err := getArgs(param)
	if err != nil {
		return "", err
	}
	return resp.EncodeArray([]string{param, v}), nil
}

func handleRPush(args []string, s *Store) (string, error) {
	if len(args) < 3 {
		return "", errors.New("RPUSH requires key and at least one value")
	}
	key := args[1]
	newItems := args[2:]
	s.mu.Lock()
	lChans, ok := s.listChans[key]
	if ok && len(lChans) > 0 {
		for range newItems {
			if len(lChans) > 0 {
				ch := lChans[0]
				lChans = lChans[1:]
				s.listChans[key] = lChans
				ch <- struct{}{}
			} else {
				break
			}
		}
	}
	s.list[key] = append(s.list[key], newItems...)
	s.listChans[key] = lChans
	s.mu.Unlock()
	return resp.EncodeInt(len(s.list[key])), nil
}

func handleLPush(args []string, s *Store) (string, error) {
	if len(args) < 3 {
		return "", errors.New("LPUSH requires key and at least one value")
	}
	key := args[1]
	s.mu.Lock()
	existing := s.list[key]
	newItems := args[2:]
	reversed := make([]string, len(newItems))
	for i, v := range newItems {
		reversed[len(newItems)-1-i] = v
	}
	s.list[key] = append(reversed, existing...)
	lChans, ok := s.listChans[key]
	if ok && len(lChans) > 0 {
		for range newItems {
			if len(lChans) > 0 {
				ch := lChans[0]
				lChans = lChans[1:]
				ch <- struct{}{}
			} else {
				break
			}
		}
	}
	s.listChans[key] = lChans
	s.mu.Unlock()
	return resp.EncodeInt(len(s.list[key])), nil
}

func handleLRange(args []string, s *Store) (string, error) {
	if len(args) < 4 {
		return "", errors.New("LRANGE requires key, start, and stop")
	}
	key := args[1]
	st, err := strconv.Atoi(args[2])
	if err != nil {
		return "", errors.New("start range not an integer")
	}
	end, err := strconv.Atoi(args[3])
	if err != nil {
		return "", errors.New("end range not an integer")
	}
	rlist := s.list[key]
	if len(rlist) == 0 {
		return resp.EncodeArray([]string{}), nil
	}
	st, end, valid := normalizeLrange(st, end, len(rlist))
	if !valid {
		return resp.EncodeArray([]string{}), nil
	}
	return resp.EncodeArray(rlist[st:end]), nil
}

func handleLLen(args []string, s *Store) (string, error) {
	if len(args) < 2 {
		return "", errors.New("LLEN requires a key")
	}
	return resp.EncodeInt(len(s.list[args[1]])), nil
}

func handleLPop(args []string, s *Store) (string, error) {
	if len(args) < 2 {
		return "", errors.New("LPOP requires a key")
	}
	key := args[1]
	valArr, ok := s.list[key]
	if !ok || len(valArr) == 0 {
		return resp.NULL, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(args) == 2 {
		val := valArr[0]
		s.list[key] = valArr[1:]
		return resp.EncodeBulkString(val), nil
	}
	count, err := strconv.Atoi(args[2])
	if err != nil {
		return "", errors.New("LPOP count is not an integer")
	}
	if count > len(valArr) {
		count = len(valArr)
	}
	val := valArr[:count]
	s.list[key] = valArr[count:]
	return resp.EncodeArray(val), nil
}

func handleBLPop(args []string, s *Store) (string, error) {
	if len(args) < 2 {
		return "", errors.New("B2LPOP requires a key")
	}
	key := args[1]
	var timeout int
	if len(args) > 2 {
		tt, err := strconv.Atoi(args[2])
		if err != nil {
			return "", errors.New("invalid timeout")
		}
		timeout = tt
	}
	s.mu.Lock()
	valArr, ok := s.list[key]
	if ok && len(valArr) > 0 {
		val := valArr[0]
		s.list[key] = valArr[1:]
		s.mu.Unlock()
		return resp.EncodeArray([]string{key, val}), nil
	}
	ch := make(chan struct{}, 1)
	s.listChans[key] = append(s.listChans[key], ch)
	s.mu.Unlock()
	if timeout > 0 {
		timeoutCh := time.After(time.Duration(timeout) * time.Second)
		for {
			select {
			case <-ch:
				s.mu.Lock()
				valArr := s.list[key]
				if len(valArr) < 1 {
					s.listChans[key] = append(s.listChans[key], ch)
					s.mu.Unlock()
					continue
				}
				val := valArr[0]
				s.list[key] = valArr[1:]
				s.mu.Unlock()
				return resp.EncodeArray([]string{key, val}), nil
			case <-timeoutCh:
				s.mu.Lock()
				chArr := s.listChans[key]
				if len(chArr) > 0 {
					for i, cha := range chArr {
						if cha == ch {
							chArr = append(chArr[:i], chArr[i+1:]...)
							s.listChans[key] = chArr
							break
						}
					}
				}
				s.mu.Unlock()
				return resp.NULL, nil
			}
		}
	} else {
		for range ch {
			s.mu.Lock()
			valArr := s.list[key]
			if len(valArr) < 1 {
				s.listChans[key] = append(s.listChans[key], ch)
				s.mu.Unlock()
				continue
			}
			val := valArr[0]
			s.list[key] = valArr[1:]
			s.mu.Unlock()
			return resp.EncodeArray([]string{key, val}), nil
		}
	}
	return resp.NULL, nil
}

func parseResp(r *RESP) (interface{}, error) {
	switch r.respStr[r.idx] {
	case '*':
		endIdx := strings.Index(r.respStr[r.idx:], "\r\n")
		arrLen, err := strconv.Atoi(r.respStr[r.idx+1 : r.idx+endIdx])
		if err != nil {
			return nil, err
		}
		res := make([]interface{}, arrLen)
		r.idx += endIdx + 2
		for i := 0; i < arrLen; i++ {
			res[i], err = parseResp(r)
			if err != nil {
				return nil, err
			}
		}
		return res, nil
	case '$':
		endIdx := strings.Index(r.respStr[r.idx:], "\r\n")
		strLen, err := strconv.Atoi(r.respStr[r.idx+1 : r.idx+endIdx])
		if err != nil {
			return nil, err
		}
		str := r.respStr[r.idx+endIdx+2 : r.idx+endIdx+2+strLen]
		r.idx += endIdx + 4 + strLen
		return str, nil
	}
	return nil, errors.New("invalid resp string")
}

func normalizeLrange(st, end, length int) (int, int, bool) {
	if st < 0 {
		st = st + length
	}
	if end < 0 {
		end = end + length
	}
	if st < 0 {
		st = 0
	}
	if st >= length || st > end {
		return 0, 0, false
	}
	end = min(end, length-1)
	return st, end + 1, true
}

func interfaceToString(interfArr []any) ([]string, error) {
	strArr := make([]string, 0, len(interfArr))
	for _, s := range interfArr {
		st, ok := s.(string)
		if !ok {
			return nil, errors.New("non string value found")
		}
		trimmed := strings.TrimSpace(st)
		if len(trimmed) < 1 {
			continue
		}
		strArr = append(strArr, trimmed)
	}
	return strArr, nil
}

func getArgs(arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if len(arg) < 1 {
		return "", errors.New("invalid argument")
	}
	flag := "--" + arg
	for i, s := range os.Args {
		if strings.TrimSpace(s) == flag && len(os.Args) > i+1 {
			return os.Args[i+1], nil
		}
	}
	return "", errors.New("argument not found: " + arg)
}
