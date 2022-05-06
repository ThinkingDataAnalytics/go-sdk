package thinkingdata

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const (
	DATE_FORMAT = "2006-01-02 15:04:05.000"
	KEY_PATTERN = "^[a-zA-Z#][A-Za-z0-9_]{0,49}$"
)

var keyPattern, _ = regexp.Compile(KEY_PATTERN)

func mergeProperties(target, source map[string]interface{}) {
	for k, v := range source {
		target[k] = v
	}
}

func extractTime(p map[string]interface{}) string {
	if t, ok := p["#time"]; ok {
		delete(p, "#time")
		switch v := t.(type) {
		case string:
			return v
		case time.Time:
			return v.Format(DATE_FORMAT)
		default:
			return time.Now().Format(DATE_FORMAT)
		}
	}

	return time.Now().Format(DATE_FORMAT)
}

func extractStringProperty(p map[string]interface{}, key string) string {
	if t, ok := p[key]; ok {
		delete(p, key)
		v, ok := t.(string)
		if !ok {
			fmt.Fprintln(os.Stderr, "Invalid data type for "+key)
		}
		return v
	}
	return ""
}

func isNotNumber(v interface{}) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
	case float32, float64:
	default:
		return true
	}
	return false
}

func formatProperties(d *Data) error {

	if d.EventName != "" {
		matched := checkPattern([]byte(d.EventName))
		if !matched {
			return errors.New("Invalid event name: " + d.EventName)
		}
	}

	if d.Properties != nil {
		for k, v := range d.Properties {
			isMatch := checkPattern([]byte(k))
			if !isMatch {
				return errors.New("Invalid property key: " + k)
			}

			if d.Type == UserAdd && isNotNumber(v) {
				return errors.New("Invalid property value: only numbers is supported by UserAdd")
			}
		}
	}

	return nil
}

func isNotArrayOrSlice(v interface{}) bool {
	typeOf := reflect.TypeOf(v)
	switch typeOf.Kind() {
	case reflect.Array:
	case reflect.Slice:
	default:
		return true
	}
	return false
}

func checkPattern(name []byte) bool {
	return keyPattern.Match(name)
}

func parseTime(input []byte) string {
	var re = regexp.MustCompile(`(((\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})(?:\.(\d{3}))\d+)?)(Z|[\+-]\d{2}:\d{2})`)
	var substitution = "$3 $4.$5"
	for re.Match(input) {
		input = re.ReplaceAll(input, []byte(substitution))
	}
	return string(input)
}

// 创建一把全局锁
var uuidLock = new(sync.Mutex)

// generateUUID 创建一个 v4 版本的 uuid
func generateUUID() string {
	// 设置随机种子
	rand.Seed(time.Now().UnixNano())

	uuidLock.Lock()

	// 模版切片
	template := []byte("xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx")
	// 最终的结果切片
	uuidSlice := make([]byte, 36)

	// 去除空白的字符，否则写入文件时候，头部会出现连续的空白字符
	uuidSlice = bytes.ReplaceAll(uuidSlice, []byte{0}, []byte{32})
	uuidSlice = bytes.TrimSpace(uuidSlice)

	// 遍历每一个byte，进行判断与替换
	for _, v := range template {
		r := rand.Intn(16)
		if v == 'x' || v == 'y' {
			if v == 'y' {
				r = r&0x3 | 0x8
			}
			// 随机数转换为16进制字符串，再转换为byte
			v = []byte(strconv.FormatInt(int64(r), 16))[0]
		}
		uuidSlice = append(uuidSlice, v)
	}

	uuidLock.Unlock()

	return string(uuidSlice)
}
