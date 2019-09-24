# ThinkingData Analytics Golang Library

ThinkingData Analytics Golang Library 是数数科技提供给客户，方便客户导入用户数据的 Golang 接口实现。如需了解详细信息，请参考 [数数科技官方网站](https://www.thinkingdata.cn).

### 一、集成 SDK

#### 1. 安装 SDK

```sh
# 获取 SDK
go get github.com/ThinkingDataAnalytics/go-sdk/thinkingdata

# 更新 SDK
go get -u github.com/ThinkingDataAnalytics/go-sdk/thinkingdata
```
#### 2. 创建 Consumer
首先在代码文件开头引入 `thinkingdata`:
```go
// import thinkingdata sdk
import	"github.com/ThinkingDataAnalytics/go-sdk/thinkingdata"
```

为使用 SDK 上传数据，需首先创建 `TDAnalytics` 实例. 创建 `TDAnalytics` 实例需要传入实现了 `Consumer` 接口的结构体. `Consumer` 的定义如下
```go
// Consumer 为数据实现 IO 操作（写入磁盘或者发送到接收端）
type Consumer interface {
	Add(d Data) error
	Flush() error
	Close() error
}
```

`thinkingdata` 包提供了 `Consuemr` 的三种实现:

**(1) LogConsumer**: 将数据实时写入本地文件，文件以天/小时切分，并需要与 LogBus 搭配使用进行数据上传 
```go
// 创建按天切分的 LogConsumer, 不设置单个日志上限
consumer, err := thinkingdata.NewLogConsumer("/path/to/data", thinkingdata.ROTATE_DAILY)

// 创建按小时切分的 LogConsumer, 不设置单个日志上限
consumer, err := thinkingdata.NewLogConsumer("/path/to/data", thinkingdata.ROTATE_HOURLY)

// 创建按天切分的 LogConsumer，并设置单个日志文件上限为 10 G
consumer, err := thinkingdata.NewLogConsumerWithFileSize("/path/to/data", thinkingdata.ROTATE_DAILY, 10 * 1024)
```
传入的参数为写入本地的文件夹地址，您只需将 LogBus 的监听文件夹地址设置为此处的地址，即可使用 LogBus 进行数据的监听上传.

**(2) BatchConsumer**: 批量实时地向 TA 服务器传输数据，不需要搭配传输工具，不建议在生产环境中使用，不支持多线程
```go
// 创建 BatchConsumer, 指定接收端地址、APP ID
consumer, err := thinkingdata.NewBatchConsumer("SERVER_URL", "APP_ID")
```
SERVER_URL 为数据接收端的 URL，APP_ID 为您的项目的 APP ID.

BatchConsumer 会先将数据存放在缓冲区中，当数据条数超过设定的值(batchSize, 默认为20)，触发上报. 您也可以在创建 BatchConsumer 的时候指定 batchSize:
```go
// 创建 BatchConsumer, 指定接收端地址、APP ID、缓冲区大小
consumer, err := thinkingdata.NewBatchConsumerWithBatchSize("SERVER_URL", "APP_ID", 50)
```

**(3) DebugConsumer**: 逐条实时向 TA 服务器传输数据，当数据格式错误时会返回详细的错误信息。建议先使用 DebugConsumer 校验数据格式
```go
consumer, _ := thinkingdata.NewDebugConsumer("SERVER_URL", "APP_ID")
```

#### 3. 创建 SDK 实例
传入创建好的 consumer，即可得到对应的 TDAnalytics 实例:
```go
ta, err := thinkingdata.New(consumer)
```
后续即可使用 ta 的接口来上报数据.

### 使用示例

#### 1. 发送事件
您可以调用 track 来上传事件，建议您根据先前梳理的文档来设置事件的属性以及发送信息的条件，此处以玩家付费作为范例：
```go

// 设置事件属性
properties := map[string]interface{}{
    // 系统预置属性, 可选. "#time" 属性是系统预置属性，传入 time.Time 对象，表示事件发生的时间
    // 如果不填入该属性，则默认使用系统当前时间
	"#time": time.Now().UTC(),
    // 系统预置属性, 可选. 如果服务端中能获取用户 IP 地址, 并填入该属性
    // 数数会自动根据 IP 地址解析用户的省份、城市信息
    "#ip": "123.123.123.123",
    // 用户自定义属性, 字符串类型
    "prop_string": "abcdefg",
    // 用户自定义属性, 数值类型
    "prop_num": 56.56,
    // 用户自定义属性, bool 类型
    "prop_bool": true,
    // 用户自定义属性, time.Time 类型
	"prop_date": time.Now(),
}

account_id := "user_account_id" // 账号 ID
distinct_id := "ABCDEF123456AGDCDD" // 访客 ID

// 上报事件名为 TEST_EVENT 的事件. account_id 和 distinct_id 不能同时为空
ta.Track(account_id, distinct_id, "TEST_EVENT", properties)
```
参数说明：
* 事件的名称只能以字母开头，可包含数字，字母和下划线“_”，长度最大为 50 个字符，对字母大小写不敏感
* 事件的属性是 map 类型，其中每个元素代表一个属性
* 事件属性的 Key 值为属性的名称，为 string 类型，规定只能以字母开头，包含数字，字母和下划线“_”，长度最大为 50 个字符，对字母大小写不敏感
* 事件属性的 Value 值为该属性的值，支持支持 string、数值类型、bool、time.Time

#### 2. 设置公共事件属性
公共事件属性是每个事件都会包含的属性.
```go
// 设置公共事件属性
ta.SetSuperProperties(map[string]interface{}{
	"SUPER_TIME":   time.Now(),
	"SUPER_BOOL":   true,
	"SUPER_STRING": "hello",
	"SUPER_NUM":    15.6,
})

// 清空公共事件属性
ta.ClearSuperProperties()
```
设置公共事件属性时，会覆盖同名的公共事件属性. 如果不存在同名属性，则添加该属性. 可以通过接口获得当前公共事件属性:
```go
currentSuperProperties := ta.GetSuperProperties()
```

#### 3. 设置用户属性
对于一般的用户属性，您可以调用 UserSet 来进行设置. 使用该接口上传的属性将会覆盖原有的属性值，如果之前不存在该用户属性，则会新建该用户属性:
```go
ta.UserSet(account_id, distinct_id, map[string]interface{}{
	"USER_STRING": "some message",
	"USER_DATE":   time.Now(),
})
```
如果您要上传的用户属性只要设置一次，则可以调用 userSetOnce 来进行设置，当该属性之前已经有值的时候，将会忽略这条信息:
```go
ta.UserSetOnce(account_id, distinct_id, map[string]interface{}{
	"USER_STRING": "some message",
	"USER_DATE":   time.Now(),
})
```
当您要上传数值型的属性时，可以调用 UserAdd 来对该属性进行累加操作，如果该属性还未被设置，则会赋值 0 后再进行计算:
```go
ta.UserAdd(account_id, distinct_id, map[string]interface{}{
	"Amount": 50,
})
```
如果您要删除某个用户，可以调用 UserDelete 将这名用户删除. 之后您将无法再查询该用户的用户属性，但该用户产生的事件仍然可以被查询到:
```go
ta.UserDelete(account_id, distinct_id)
```

#### 4. 立即进行数据 IO
此操作与具体的 Consumer 实现有关. 在收到数据时, Consumer 可以先将数据存放在缓冲区, 并在一定情况下触发真正的数据 IO 操作, 以提高整体性能. 在某些情况下需要立即提交数据，可以调用 Flush 接口:
```go
// 立即提交数据到相应的接收端
ta.Flush()
```

#### 5. 关闭 SDK
请在退出程序前调用本接口，以避免缓存内的数据丢失:
```go
// 关闭并退出 SDK
ta.Close()
```
