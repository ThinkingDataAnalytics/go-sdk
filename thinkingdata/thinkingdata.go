package thinkingdata

import (
	"errors"
	"sync"
)

const (
	Track          = "track"
	TrackUpdate    = "track_update"
	TrackOverwrite = "track_overwrite"
	UserSet        = "user_set"
	UserUnset      = "user_unset"
	UserSetOnce    = "user_setOnce"
	UserAdd        = "user_add"
	UserAppend     = "user_append"
	UserDel        = "user_del"

	SdkVersion = "1.4.2"
	LibName    = "Golang"
)

// Data 数据信息
type Data struct {
	AccountId    string                 `json:"#account_id,omitempty"`
	DistinctId   string                 `json:"#distinct_id,omitempty"`
	Type         string                 `json:"#type"`
	Time         string                 `json:"#time"`
	EventName    string                 `json:"#event_name,omitempty"`
	EventId      string                 `json:"#event_id,omitempty"`
	FirstCheckId string                 `json:"#first_check_id,omitempty"`
	Ip           string                 `json:"#ip,omitempty"`
	UUID         string                 `json:"#uuid,omitempty"`
	Properties   map[string]interface{} `json:"properties"`
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

// New 初始化 TDAnalytics
func New(c Consumer) TDAnalytics {
	return TDAnalytics{consumer: c,
		superProperties: make(map[string]interface{}),
		mutex:           new(sync.RWMutex)}
}

// GetSuperProperties 返回公共事件属性
func (ta *TDAnalytics) GetSuperProperties() map[string]interface{} {
	result := make(map[string]interface{})
	ta.mutex.RLock()
	mergeProperties(result, ta.superProperties)
	ta.mutex.RUnlock()
	return result
}

// SetSuperProperties 设置公共事件属性
func (ta *TDAnalytics) SetSuperProperties(superProperties map[string]interface{}) {
	ta.mutex.Lock()
	mergeProperties(ta.superProperties, superProperties)
	ta.mutex.Unlock()
}

// ClearSuperProperties 清除公共事件属性
func (ta *TDAnalytics) ClearSuperProperties() {
	ta.mutex.Lock()
	ta.superProperties = make(map[string]interface{})
	ta.mutex.Unlock()
}

// Track 追踪一个事件
func (ta *TDAnalytics) Track(accountId, distinctId, eventName string, properties map[string]interface{}) error {
	return ta.track(accountId, distinctId, Track, eventName, "", properties)
}

func (ta *TDAnalytics) TrackUpdate(accountId, distinctId, eventName, eventId string, properties map[string]interface{}) error {
	return ta.track(accountId, distinctId, TrackUpdate, eventName, eventId, properties)
}

func (ta *TDAnalytics) TrackOverwrite(accountId, distinctId, eventName, eventId string, properties map[string]interface{}) error {
	return ta.track(accountId, distinctId, TrackOverwrite, eventName, eventId, properties)
}

func (ta *TDAnalytics) track(accountId, distinctId, dataType, eventName, eventId string, properties map[string]interface{}) error {
	if len(eventName) == 0 {
		return errors.New("the event name must be provided")
	}

	if len(eventId) == 0 && dataType != Track {
		return errors.New("the event id must be provided")
	}

	p := ta.GetSuperProperties()
	p["#lib"] = LibName
	p["#lib_version"] = SdkVersion

	mergeProperties(p, properties)

	return ta.add(accountId, distinctId, dataType, eventName, eventId, p)
}

// UserSet 设置用户属性. 如果同名属性已存在，则用传入的属性覆盖同名属性.
func (ta *TDAnalytics) UserSet(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserSet, properties)
}

// UserUnset 删除用户属性
func (ta *TDAnalytics) UserUnset(accountId string, distinctId string, s []string) error {
	if len(s) == 0 {
		return errors.New("invalid params for UserUnset: properties is nil")
	}
	prop := make(map[string]interface{})
	for _, v := range s {
		prop[v] = 0
	}
	return ta.user(accountId, distinctId, UserUnset, prop)
}

// UserSetOnce 设置用户属性. 不会覆盖同名属性.
func (ta *TDAnalytics) UserSetOnce(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserSetOnce, properties)
}

// UserAdd 对数值类型的属性做累加操作
func (ta *TDAnalytics) UserAdd(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserAdd, properties)
}

// UserAppend 对数组类型的属性做追加加操作
func (ta *TDAnalytics) UserAppend(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserAppend, properties)
}

// UserDelete 删除用户数据, 之后无法查看用户属性, 但是之前已经入库的事件数据不会被删除. 此操作不可逆
func (ta *TDAnalytics) UserDelete(accountId string, distinctId string) error {
	return ta.user(accountId, distinctId, UserDel, nil)
}

func (ta *TDAnalytics) user(accountId, distinctId, dataType string, properties map[string]interface{}) error {
	if properties == nil && dataType != UserDel {
		return errors.New("invalid params for " + dataType + ": properties is nil")
	}
	p := make(map[string]interface{})
	mergeProperties(p, properties)
	return ta.add(accountId, distinctId, dataType, "", "", p)
}

// Flush 立即开始数据 IO 操作
func (ta *TDAnalytics) Flush() error {
	return ta.consumer.Flush()
}

// Close 关闭 TDAnalytics
func (ta *TDAnalytics) Close() error {
	return ta.consumer.Close()
}

func (ta *TDAnalytics) add(accountId, distinctId, dataType, eventName, eventId string, properties map[string]interface{}) error {
	if len(accountId) == 0 && len(distinctId) == 0 {
		return errors.New("invalid paramters: account_id and distinct_id cannot be empty at the same time")
	}

	// 获取 properties 中 #ip 值, 如不存在则返回 ""
	ip := extractStringProperty(properties, "#ip")

	// 获取 properties 中 #time 值, 如不存在则返回当前时间
	eventTime := extractTime(properties)

	firstCheckId := extractStringProperty(properties, "#first_check_id")

	//如果上传#uuid， 只支持UUID标准格式xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx的string类型
	uuid := extractStringProperty(properties, "#uuid")

	data := Data{
		AccountId:    accountId,
		DistinctId:   distinctId,
		Type:         dataType,
		Time:         eventTime,
		EventName:    eventName,
		EventId:      eventId,
		FirstCheckId: firstCheckId,
		Ip:           ip,
		UUID:         uuid,
		Properties:   properties,
	}

	// 检查数据格式, 并将时间类型数据转为符合格式要求的字符串
	err := formatProperties(&data)
	if err != nil {
		return err
	}

	return ta.consumer.Add(data)
}
