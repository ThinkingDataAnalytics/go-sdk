package thinkingdata

import (
	"errors"
	"sync"
)

const (
	Track          = "track"
	TrackUpdate    = "track_update"    // 可更新事件
	TrackOverwrite = "track_overwrite" // 可重写事件
	UserSet        = "user_set"
	UserUnset      = "user_unset"
	UserSetOnce    = "user_setOnce"
	UserAdd        = "user_add"
	UserAppend     = "user_append"
	UserUniqAppend = "user_uniq_append" // 追加用户属性，支持去重，再进行入库
	UserDel        = "user_del"

	SdkVersion = "1.6.6"
	LibName    = "Golang"
)

// Data 数据信息
type Data struct {
	IsComplex    bool                   `json:"-"` // 标记传入的 property 是否是复杂数据类型
	AccountId    string                 `json:"#account_id,omitempty"`
	DistinctId   string                 `json:"#distinct_id,omitempty"`
	Type         string                 `json:"#type"`
	Time         string                 `json:"#time"`
	EventName    string                 `json:"#event_name,omitempty"`
	EventId      string                 `json:"#event_id,omitempty"`
	FirstCheckId string                 `json:"#first_check_id,omitempty"`
	Ip           string                 `json:"#ip,omitempty"`
	UUID         string                 `json:"#uuid,omitempty"`
	AppId        string                 `json:"#app_id,omitempty"`
	Properties   map[string]interface{} `json:"properties"`
}

// Consumer 为数据实现 IO 操作（写入磁盘或者发送到接收端）
type Consumer interface {
	Add(d Data) error
	Flush() error
	Close() error
	IsStringent() bool // 是否是严格模式
}

type TDAnalytics struct {
	consumer               Consumer
	superProperties        map[string]interface{}
	mutex                  *sync.RWMutex
	dynamicSuperProperties func() map[string]interface{}
}

// New 初始化 TDAnalytics
func New(c Consumer) TDAnalytics {
	Logger("初始化成功")
	return TDAnalytics{
		consumer:        c,
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

// SetDynamicSuperProperties 设置动态公共事件属性
func (ta *TDAnalytics) SetDynamicSuperProperties(action func() map[string]interface{}) {
	ta.mutex.Lock()
	ta.dynamicSuperProperties = action
	ta.mutex.Unlock()
}

// GetDynamicSuperProperties 返回动态公共事件属性
func (ta *TDAnalytics) GetDynamicSuperProperties() map[string]interface{} {
	result := make(map[string]interface{})
	ta.mutex.RLock()
	if ta.dynamicSuperProperties != nil {
		mergeProperties(result, ta.dynamicSuperProperties())
	}
	ta.mutex.RUnlock()
	return result
}

// Track 追踪一个事件
func (ta *TDAnalytics) Track(accountId, distinctId, eventName string, properties map[string]interface{}) error {
	return ta.track(accountId, distinctId, Track, eventName, "", properties)
}

// TrackFirst 首次事件
func (ta *TDAnalytics) TrackFirst(accountId, distinctId, eventName, firstCheckId string, properties map[string]interface{}) error {
	if len(firstCheckId) == 0 {
		msg := "the 'firstCheckId' must be provided"
		Logger(msg)
		return errors.New(msg)
	}
	properties["#first_check_id"] = firstCheckId
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
		msg := "the event name must be provided"
		Logger(msg)
		return errors.New(msg)
	}

	// 只有eventType == Track 的时候才不需要eventId
	if len(eventId) == 0 && dataType != Track {
		msg := "the event id must be provided"
		Logger(msg)
		return errors.New(msg)
	}

	// 获取设置的公共属性
	p := ta.GetSuperProperties()

	// 动态公共属性
	dynamicSuperProperties := ta.GetDynamicSuperProperties()

	ta.mutex.Lock()
	// 组合动态公共属性
	mergeProperties(p, dynamicSuperProperties)
	// 预制属性优先级高于公共属性
	p["#lib"] = LibName
	p["#lib_version"] = SdkVersion
	// 自定义属性
	mergeProperties(p, properties)
	ta.mutex.Unlock()

	return ta.add(accountId, distinctId, dataType, eventName, eventId, p)
}

// UserSet 设置用户属性. 如果同名属性已存在，则用传入的属性覆盖同名属性.
func (ta *TDAnalytics) UserSet(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserSet, properties)
}

// UserUnset 删除用户属性
func (ta *TDAnalytics) UserUnset(accountId string, distinctId string, s []string) error {
	if len(s) == 0 {
		msg := "invalid params for UserUnset: properties is nil"
		Logger(msg)
		return errors.New(msg)
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

// UserUniqAppend 对数组类型的属性做追加加操作，对于重复元素进行去重处理
func (ta *TDAnalytics) UserUniqAppend(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserUniqAppend, properties)
}

// UserDelete 删除用户数据, 之后无法查看用户属性, 但是之前已经入库的事件数据不会被删除. 此操作不可逆
func (ta *TDAnalytics) UserDelete(accountId string, distinctId string) error {
	return ta.user(accountId, distinctId, UserDel, nil)
}

func (ta *TDAnalytics) user(accountId, distinctId, dataType string, properties map[string]interface{}) error {
	if properties == nil && dataType != UserDel {
		msg := "invalid params for " + dataType + ": properties is nil"
		Logger(msg)
		return errors.New(msg)
	}
	p := make(map[string]interface{})
	ta.mutex.Lock()
	mergeProperties(p, properties)
	ta.mutex.Unlock()
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
		msg := "invalid parameters: account_id and distinct_id cannot be empty at the same time"
		Logger(msg)
		return errors.New(msg)
	}

	// 获取 properties 中 #ip 值, 如不存在则返回 ""
	ip := extractStringProperty(properties, "#ip")

	// 获取 properties 中 #app_id 值, 如不存在则返回 ""
	appId := extractStringProperty(properties, "#app_id")

	// 获取 properties 中 #time 值, 如不存在则返回当前时间
	eventTime := extractTime(properties)

	firstCheckId := extractStringProperty(properties, "#first_check_id")

	//如果上传#uuid， 只支持UUID标准格式xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx的string类型
	uuid := extractStringProperty(properties, "#uuid")
	if len(uuid) == 0 {
		uuid = generateUUID()
	}

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

	if len(appId) > 0 {
		data.AppId = appId
	}

	// 检查数据格式, 并将时间类型数据转为符合格式要求的字符串
	err := formatProperties(&data, ta)
	if err != nil {
		return err
	}

	return ta.consumer.Add(data)
}
