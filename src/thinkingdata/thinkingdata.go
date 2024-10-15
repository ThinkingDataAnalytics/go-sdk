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
	UserUniqAppend = "user_uniq_append"
	UserDel        = "user_del"

	SdkVersion = "2.0.3"
	LibName    = "Golang"
)

type Data struct {
	IsComplex    bool                   `json:"-"` // properties are nested or not
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

// TDConsumer define operation interface
type TDConsumer interface {
	Add(d Data) error
	Flush() error
	Close() error
	IsStringent() bool // check data or not.
}

type TDAnalytics struct {
	consumer               TDConsumer
	superProperties        map[string]interface{}
	mutex                  *sync.RWMutex
	dynamicSuperProperties func() map[string]interface{}
}

// New init SDK
func New(c TDConsumer) TDAnalytics {
	tdLogInfo("init SDK success")
	return TDAnalytics{
		consumer:        c,
		superProperties: make(map[string]interface{}),
		mutex:           new(sync.RWMutex),
	}
}

// GetSuperProperties get common properties
func (ta *TDAnalytics) GetSuperProperties() map[string]interface{} {
	result := make(map[string]interface{})
	ta.mutex.Lock()
	mergeProperties(result, ta.superProperties)
	ta.mutex.Unlock()
	return result
}

// SetSuperProperties set common properties
func (ta *TDAnalytics) SetSuperProperties(superProperties map[string]interface{}) {
	ta.mutex.Lock()
	mergeProperties(ta.superProperties, superProperties)
	ta.mutex.Unlock()
}

// ClearSuperProperties clear common properties
func (ta *TDAnalytics) ClearSuperProperties() {
	ta.mutex.Lock()
	ta.superProperties = make(map[string]interface{})
	ta.mutex.Unlock()
}

// SetDynamicSuperProperties set common properties dynamically.
// not recommend to add the operation which with a lot of computation
func (ta *TDAnalytics) SetDynamicSuperProperties(action func() map[string]interface{}) {
	ta.mutex.Lock()
	ta.dynamicSuperProperties = action
	ta.mutex.Unlock()
}

// GetDynamicSuperProperties dynamic common properties
func (ta *TDAnalytics) GetDynamicSuperProperties() map[string]interface{} {
	result := make(map[string]interface{})
	ta.mutex.RLock()
	if ta.dynamicSuperProperties != nil {
		mergeProperties(result, ta.dynamicSuperProperties())
	}
	ta.mutex.RUnlock()
	return result
}

// Track report ordinary event
func (ta *TDAnalytics) Track(accountId, distinctId, eventName string, properties map[string]interface{}) error {
	return ta.track(accountId, distinctId, Track, eventName, "", properties)
}

// TrackFirst report first event
func (ta *TDAnalytics) TrackFirst(accountId, distinctId, eventName, firstCheckId string, properties map[string]interface{}) error {
	if len(firstCheckId) == 0 {
		msg := "the 'firstCheckId' must be provided"
		tdLogInfo(msg)
		return errors.New(msg)
	}
	p := make(map[string]interface{})
	mergeProperties(p, properties)
	p["#first_check_id"] = firstCheckId
	return ta.track(accountId, distinctId, Track, eventName, "", p)
}

// TrackUpdate report updatable event
func (ta *TDAnalytics) TrackUpdate(accountId, distinctId, eventName, eventId string, properties map[string]interface{}) error {
	return ta.track(accountId, distinctId, TrackUpdate, eventName, eventId, properties)
}

// TrackOverwrite report overridable event
func (ta *TDAnalytics) TrackOverwrite(accountId, distinctId, eventName, eventId string, properties map[string]interface{}) error {
	return ta.track(accountId, distinctId, TrackOverwrite, eventName, eventId, properties)
}

func (ta *TDAnalytics) track(accountId, distinctId, dataType, eventName, eventId string, properties map[string]interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			tdLogError("%+v\ndata: %+v", r, properties)
		}
	}()

	if len(eventName) == 0 {
		msg := "the event name must be provided"
		tdLogError(msg)
		return errors.New(msg)
	}

	// eventId not be null unless eventType is equal Track.
	if len(eventId) == 0 && dataType != Track {
		msg := "the event id must be provided"
		tdLogError(msg)
		return errors.New(msg)
	}

	p := ta.GetSuperProperties()
	dynamicSuperProperties := ta.GetDynamicSuperProperties()

	mergeProperties(p, dynamicSuperProperties)
	// preset properties has the highest priority
	p["#lib"] = LibName
	p["#lib_version"] = SdkVersion
	// custom properties
	mergeProperties(p, properties)

	return ta.add(accountId, distinctId, dataType, eventName, eventId, p)
}

// UserSet set user properties. would overwrite existing names.
func (ta *TDAnalytics) UserSet(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserSet, properties)
}

// UserUnset clear the user properties of users.
func (ta *TDAnalytics) UserUnset(accountId string, distinctId string, s []string) error {
	if len(s) == 0 {
		msg := "invalid params for UserUnset: keys is nil"
		tdLogInfo(msg)
		return errors.New(msg)
	}
	prop := make(map[string]interface{})
	for _, v := range s {
		prop[v] = 0
	}
	return ta.user(accountId, distinctId, UserUnset, prop)
}

func (ta *TDAnalytics) UserUnsetWithProperties(accountId string, distinctId string, properties map[string]interface{}) error {
	if len(properties) == 0 {
		msg := "invalid params for UserUnset: properties is nil"
		tdLogInfo(msg)
		return errors.New(msg)
	}
	return ta.user(accountId, distinctId, UserUnset, properties)
}

// UserSetOnce set user properties, If such property had been set before, this message would be neglected.
func (ta *TDAnalytics) UserSetOnce(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserSetOnce, properties)
}

// UserAdd to accumulate operations against the property.
func (ta *TDAnalytics) UserAdd(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserAdd, properties)
}

// UserAppend to add user properties of array type.
func (ta *TDAnalytics) UserAppend(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserAppend, properties)
}

// UserUniqAppend append user properties to array type by unique.
func (ta *TDAnalytics) UserUniqAppend(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserUniqAppend, properties)
}

// UserDelete delete a user, This operation cannot be undone.
func (ta *TDAnalytics) UserDelete(accountId string, distinctId string) error {
	return ta.user(accountId, distinctId, UserDel, nil)
}

// UserDeleteWithProperties delete a user, This operation cannot be undone.
func (ta *TDAnalytics) UserDeleteWithProperties(accountId string, distinctId string, properties map[string]interface{}) error {
	return ta.user(accountId, distinctId, UserDel, properties)
}

func (ta *TDAnalytics) user(accountId, distinctId, dataType string, properties map[string]interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			tdLogError("%+v\ndata: %+v", r, properties)
		}
	}()
	if properties == nil && dataType != UserDel {
		msg := "invalid params for " + dataType + ": properties is nil"
		tdLogError(msg)
		return errors.New(msg)
	}
	p := make(map[string]interface{})
	mergeProperties(p, properties)
	return ta.add(accountId, distinctId, dataType, "", "", p)
}

// Flush report data immediately.
func (ta *TDAnalytics) Flush() error {
	return ta.consumer.Flush()
}

// Close and exit sdk
func (ta *TDAnalytics) Close() error {
	err := ta.consumer.Close()
	tdLogInfo("SDK close")
	return err
}

func (ta *TDAnalytics) add(accountId, distinctId, dataType, eventName, eventId string, properties map[string]interface{}) error {
	if len(accountId) == 0 && len(distinctId) == 0 {
		msg := "invalid parameters: account_id and distinct_id cannot be empty at the same time"
		tdLogError(msg)
		return errors.New(msg)
	}

	// get "#ip" value in properties, empty string will be return when not found.
	ip := extractStringProperty(properties, "#ip")

	// get "#app_id" value in properties, empty string will be return when not found.
	appId := extractStringProperty(properties, "#app_id")

	// get "#time" value in properties, empty string will be return when not found.
	eventTime := extractTime(properties)

	firstCheckId := extractStringProperty(properties, "#first_check_id")

	// get "#uuid" value in properties, empty string will be return when not found.
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

	err := formatProperties(&data, ta)
	if err != nil {
		return err
	}

	return ta.consumer.Add(data)
}

// Deprecated: please use TDConsumer
type Consumer interface {
	TDConsumer
}
