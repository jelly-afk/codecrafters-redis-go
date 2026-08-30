package resp

import (
	"fmt"
	"strings"
)

const (
	PONG = "+PONG\r\n"
	OK   = "+OK\r\n"
	NULL = "$-1\r\n"
)

func EncodeBulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}
func EncodeArray(arr []any) string {
	res := make([]string, 0)
	for _, val := range arr {
		switch v := val.(type) {
		case string:
			res = append(res, EncodeBulkString(v))
		}
	}
	return fmt.Sprintf("*%d\r\n%s", len(res), strings.Join(res, ""))
}
