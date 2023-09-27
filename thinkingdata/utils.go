package thinkingdata

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"time"

	"github.com/google/uuid"
)

const (
	DATE_FORMAT = "2006-01-02 15:04:05.000"
	KEY_PATTERN = "^[a-zA-Z#][A-Za-z0-9_]{0,49}$"
)

// A string of 50 letters and digits that starts with '#' or a letter
var keyPattern, _ = regexp.Compile(KEY_PATTERN)
var reRFC3339 = regexp.MustCompile(`"((\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})(?:\.(\d{3}))\d*)(Z|[\+-]\d{2}:\d{2})"`)

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

func formatProperties(d *Data, ta *TDAnalytics) error {

	if d.EventName != "" {
		matched := checkPattern([]byte(d.EventName))
		if !matched {
			msg := "invalid event name: " + d.EventName
			Logger(msg)
			return errors.New(msg)
		}
	}

	if d.Properties != nil {
		for k, v := range d.Properties {
			if ta.consumer.IsStringent() {
				isMatch := checkPattern([]byte(k))
				if !isMatch {
					msg := "invalid property key: " + k
					Logger(msg)
					return errors.New(msg)
				}
			}

			if d.Type == UserAdd && isNotNumber(v) {
				msg := "invalid property value: only numbers is supported by UserAdd"
				Logger(msg)
				return errors.New(msg)
			}

			// check value
			switch v.(type) {
			case int:
			case bool:
			case float64:
			case string:
			case time.Time:
				d.Properties[k] = v.(time.Time).Format(DATE_FORMAT)
			case []string:
				d.IsComplex = true
			default:
				d.IsComplex = true
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
	var substitution = "\"$2 $3.$4\""

	for reRFC3339.Match(input) {
		input = reRFC3339.ReplaceAll(input, []byte(substitution))
	}
	return string(input)
}

func generateUUID() string {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return ""
	}
	return newUUID.String()
}
