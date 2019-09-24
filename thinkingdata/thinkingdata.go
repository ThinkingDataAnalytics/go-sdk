package thinkingdata

import (
	"errors"
	"sync"
)

const (
	TRACK         = "track"
	USER_SET      = "user_set"
	USER_SET_ONCE = "user_setOnce"
	USER_ADD      = "user_add"
	USER_DEL      = "user_del"

	SDK_VERSION = "1.0.0"
	LIB_NAME    = "Golang"
)

// 数据信息
type Data struct {
	AccountId  string                 `json:"#account_id"`
	DistinctId string                 `json:"#distinct_id"`
	Type       string                 `json:"#type"`
	Time       string                 `json:"#time"`
	EventName  string                 `json:"#event_name"`
	Ip         string                 `json:"#ip"`
	Properties map[string]interface{} `json:"properties"`
}

// Consumer 为数据实现 IO 操作（写入磁盘或者发送到接收端）
type Consumer interface {
	Add(d Data) error
	Flush() error
	Close() error
}

type TDAnalytics struct {
	consumer        Consumer
	superProperties map[string]interface{}
	mutex           *sync.RWMutex
}

// 初始化 TDAnalytics
func New(c Consumer) TDAnalytics {
	return TDAnalytics{consumer: c,
		superProperties: make(map[string]interface{}),
		mutex:           new(sync.RWMutex)}
}

// 返回公共事件属性
func (ta *TDAnalytics) GetSuperProperties() map[string]interface{} {
	result := make(map[string]interface{})
	ta.mutex.RLock()
	mergeProperties(result, ta.superProperties)
	ta.mutex.RUnlock()
	return result
}

// 设置公共事件属性
func (ta *TDAnalytics) SetSuperProperties(superProperties map[string]interface{}) {
	ta.mutex.Lock()
	mergeProperties(ta.superProperties, superProperties)
	ta.mutex.Unlock()
}

// 清除公共事件属性
func (ta *TDAnalytics) ClearSuperProperties() {
	ta.mutex.Lock()
	ta.superProperties = make(map[string]interface{})
	ta.mutex.Unlock()
}

// 追踪一个事件
func (ta *TDAnalytics) Track(accountId string, distinctId string, eventName string, properties map[string]interface{}) error {
	p := ta.GetSuperProperties()
	p["#lib"] = LIB_NAME
	p["#lib_version"] = SDK_VERSION

	mergeProperties(p, properties)

	return ta.add(accountId, distinctId, TRACK, eventName, p)
}

// 设置用户属性. 如果同名属性已存在，则用传入的属性覆盖同名属性.
func (ta *TDAnalytics) UserSet(accountId string, distinctId string, properties map[string]interface{}) error {
	if properties == nil {
		return errors.New("Invalid params for UserSet: properties is nil")
	}

	p := make(map[string]interface{})
	mergeProperties(p, properties)
	return ta.add(accountId, distinctId, USER_SET, "", p)
}

// 设置用户属性. 不会覆盖同名属性.
func (ta *TDAnalytics) UserSetOnce(accountId string, distinctId string, properties map[string]interface{}) error {
	if properties == nil {
		return errors.New("Invalid params for UserSetOnce: properties is nil")
	}

	p := make(map[string]interface{})
	mergeProperties(p, properties)
	return ta.add(accountId, distinctId, USER_SET_ONCE, "", p)
}

// 对数值类型的属性做累加操作
func (ta *TDAnalytics) UserAdd(accountId string, distinctId string, properties map[string]interface{}) error {
	if properties == nil {
		return errors.New("Invalid params for UserAdd: properties is nil")
	}

	p := make(map[string]interface{})
	mergeProperties(p, properties)
	return ta.add(accountId, distinctId, USER_ADD, "", p)
}

// 删除用户数据, 之后无法查看用户属性, 但是之前已经入库的事件数据不会被删除. 此操作不可逆
func (ta *TDAnalytics) UserDelete(accountId string, distinctId string) error {
	return ta.add(accountId, distinctId, USER_DEL, "", nil)
}

// 立即开始数据 IO 操作
func (ta *TDAnalytics) Flush() {
	ta.consumer.Flush()
}

// 关闭 TDAnalytics
func (ta *TDAnalytics) Close() {
	ta.consumer.Close()
}

func (ta *TDAnalytics) add(accountId string, distinctId string, dataType string, eventName string, properties map[string]interface{}) error {
	if len(accountId) == 0 && len(distinctId) == 0 {
		return errors.New("Invalid paramters: account_id and distinct_id cannot be empty at the same time")
	}

	// 获取 properties 中 #ip 值, 如不存在则返回 ""
	ip := extractIp(properties)

	// 获取 properties 中 #time 值, 如不存在则返回当前时间
	eventTime := extractTime(properties)

	data := Data{
		AccountId:  accountId,
		DistinctId: distinctId,
		Type:       dataType,
		Time:       eventTime,
		EventName:  eventName,
		Ip:         ip,
		Properties: properties,
	}

	// 检查数据格式, 并将时间类型数据转为符合格式要求的字符串
	err := formatProperties(&data)
	if err != nil {
		return err
	}

	return ta.consumer.Add(data)
}
