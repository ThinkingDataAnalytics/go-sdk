package thinkingdata

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"
)

const (
	DATE_FORMAT = "2006-01-02 15:04:05.000"
	KEY_PATTERN = "^[a-zA-Z#][A-Za-z0-9_]{0,49}$"
	VALUE_MAX   = 2048
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

		v, ok := t.(time.Time)
		if !ok {
			fmt.Fprintln(os.Stderr, "Invalid data type for #time")
			return time.Now().Format(DATE_FORMAT)
		}
		return v.Format(DATE_FORMAT)
	}

	return time.Now().Format(DATE_FORMAT)
}

func extractIp(p map[string]interface{}) string {
	if t, ok := p["#ip"]; ok {
		delete(p, "#ip")

		v, ok := t.(string)
		if !ok {
			fmt.Fprintln(os.Stderr, "Invalid data type for #ip")
			return ""
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

			if d.Type == USER_ADD && isNotNumber(v) {
				return errors.New("Invalid property value: only numbers is supported by UserAdd")
			}

			//check value
			switch v.(type) {
			case bool:
			case string:
				if len(v.(string)) > VALUE_MAX {
					return errors.New("the max length of property value is 2048")
				}
			case time.Time: //only support time.Time
				d.Properties[k] = v.(time.Time).Format(DATE_FORMAT)
			default:
				if isNotNumber(v) {
					return errors.New("Invalid property value type. Supported types: numbers, string, time.Time, bool")
				}
			}
		}
	}

	return nil
}

func checkPattern(name []byte) bool {
	return keyPattern.Match(name)
}
