package thinkingdata

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// TDBatchConsumer upload data to TE by http
type TDBatchConsumer struct {
	serverUrl   string // serverUrl
	appId       string // appId
	compress    bool   // is need compress
	bufferMutex *sync.RWMutex
	cacheMutex  *sync.RWMutex // cache mutex

	buffer        []json.RawMessage
	batchSize     int                 // event count threshold for triggering a flush
	cacheBuffer   [][]json.RawMessage // buffer
	cacheCapacity int                 // buffer max count
	HttpClient    *http.Client
	closing       uint32
	closeOnce     sync.Once
	closeDone     chan struct{}
	closeErr      error // published when closeDone closes
	stopAutoFlush chan struct{}
	autoFlushDone chan struct{}
	ticker        *time.Ticker
	sendGate      chan struct{} // serializes senders, never queue admission
	emittingLogs  uint32
	timeout       time.Duration
}

type TDBatchConfig struct {
	ServerUrl     string       // serverUrl
	AppId         string       // appId
	BatchSize     int          // event count threshold for triggering a flush
	Timeout       int          // request deadline in milliseconds, including custom clients
	Compress      bool         // enable compress data
	AutoFlush     bool         // enable auto flush
	Interval      int          // auto flush spacing (second)
	CacheCapacity int          // cache event count
	HttpClient    *http.Client // Custom http client. Set this parameter when you want to use your own http client
}

const (
	DefaultTimeOut       = 30000
	DefaultBatchSize     = 20
	MaxBatchSize         = 200
	DefaultInterval      = 30
	DefaultCacheCapacity = 50
)

// NewBatchConsumer create TDBatchConsumer
func NewBatchConsumer(serverUrl string, appId string) (TDConsumer, error) {
	config := TDBatchConfig{
		ServerUrl: serverUrl,
		AppId:     appId,
		Compress:  true,
	}
	return initBatchConsumer(config)
}

// NewBatchConsumerWithBatchSize create TDBatchConsumer
// serverUrl
// appId
// batchSize: flush event count each time
func NewBatchConsumerWithBatchSize(serverUrl string, appId string, batchSize int) (TDConsumer, error) {
	config := TDBatchConfig{
		ServerUrl: serverUrl,
		AppId:     appId,
		Compress:  true,
		BatchSize: batchSize,
	}
	return initBatchConsumer(config)
}

// NewBatchConsumerWithCompress create TDBatchConsumer
// serverUrl
// appId
// compress: enable data compress
func NewBatchConsumerWithCompress(serverUrl string, appId string, compress bool) (TDConsumer, error) {
	config := TDBatchConfig{
		ServerUrl: serverUrl,
		AppId:     appId,
		Compress:  compress,
	}
	return initBatchConsumer(config)
}

func NewBatchConsumerWithConfig(config TDBatchConfig) (TDConsumer, error) {
	return initBatchConsumer(config)
}

func initBatchConsumer(config TDBatchConfig) (TDConsumer, error) {
	if config.ServerUrl == "" {
		msg := "ServerUrl not be empty"
		tdLogInfo(msg)
		return nil, errors.New(msg)
	}
	u, err := url.Parse(config.ServerUrl)
	if err != nil {
		return nil, err
	}
	u.Path = "/sync_server"

	interval, err := batchFlushInterval(config.Interval)
	if err != nil {
		return nil, err
	}

	var batchSize int
	if config.BatchSize > MaxBatchSize {
		batchSize = MaxBatchSize
	} else if config.BatchSize <= 0 {
		batchSize = DefaultBatchSize
	} else {
		batchSize = config.BatchSize
	}

	var cacheCapacity int
	if config.CacheCapacity <= 0 {
		cacheCapacity = DefaultCacheCapacity
	} else {
		cacheCapacity = config.CacheCapacity
	}

	timeout, err := batchRequestTimeout(config.Timeout)
	if err != nil {
		return nil, err
	}

	httpClient := config.HttpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	c := &TDBatchConsumer{
		serverUrl:     u.String(),
		appId:         config.AppId,
		compress:      config.Compress,
		bufferMutex:   new(sync.RWMutex),
		cacheMutex:    new(sync.RWMutex),
		batchSize:     batchSize,
		buffer:        make([]json.RawMessage, 0, batchSize),
		cacheCapacity: cacheCapacity,
		cacheBuffer:   make([][]json.RawMessage, 0, cacheCapacity),
		HttpClient:    httpClient,
		closeDone:     make(chan struct{}),
		stopAutoFlush: make(chan struct{}),
		autoFlushDone: make(chan struct{}),
		sendGate:      make(chan struct{}, 1),
		timeout:       timeout,
	}

	if config.AutoFlush {
		c.ticker = time.NewTicker(interval)
		go c.runAutoFlush()
	} else {
		close(c.autoFlushDone)
	}

	tdLogInfo("Mode: batch consumer, appId: %s, serverUrl: %s", c.appId, c.serverUrl)

	return c, nil
}

var errBatchConsumerClosed = errors.New("batch consumer has been closed")

func (c *TDBatchConsumer) runAutoFlush() {
	defer close(c.autoFlushDone)
	defer c.ticker.Stop()
	for {
		select {
		case <-c.stopAutoFlush:
			return
		case <-c.ticker.C:
			if atomic.LoadUint32(&c.closing) != 0 {
				return
			}
			_ = c.timerFlush()
		}
	}
}

// Validate seconds before multiplication: overflow can produce a negative or
// unexpectedly small positive duration. Zero retains the default interval.
func batchFlushInterval(seconds int) (time.Duration, error) {
	if seconds == 0 {
		seconds = DefaultInterval
	}
	const maxSeconds = int64((1<<63 - 1) / time.Second)
	if seconds < 0 || int64(seconds) > maxSeconds {
		return 0, fmt.Errorf("invalid Interval %d: must be between 0 and %d seconds", seconds, maxSeconds)
	}
	return time.Duration(seconds) * time.Second, nil
}

func batchRequestTimeout(milliseconds int) (time.Duration, error) {
	if milliseconds == 0 {
		milliseconds = DefaultTimeOut
	}
	const maxMilliseconds = int64((1<<63 - 1) / time.Millisecond)
	if milliseconds < 0 || int64(milliseconds) > maxMilliseconds {
		return 0, fmt.Errorf("invalid Timeout %d: must be between 0 and %d milliseconds", milliseconds, maxMilliseconds)
	}
	return time.Duration(milliseconds) * time.Millisecond, nil
}

// Add accepts an immutable event snapshot. Threshold-triggered sends remain
// synchronous; producers can enqueue while another request is in flight.
func (c *TDBatchConsumer) Add(d Data) error {
	if atomic.LoadUint32(&c.closing) != 0 {
		return errBatchConsumerClosed
	}
	var jsonBytes []byte
	err := withPanicRecovery(func() error {
		var err error
		jsonBytes, err = json.Marshal(d)
		return err
	})
	if err != nil {
		return err
	}
	c.bufferMutex.Lock()
	if atomic.LoadUint32(&c.closing) != 0 {
		c.bufferMutex.Unlock()
		return errBatchConsumerClosed
	}
	c.buffer = append(c.buffer, json.RawMessage(jsonBytes))
	ready := len(c.buffer) >= c.batchSize
	c.bufferMutex.Unlock()
	if GetLogLevel() >= TDLogLevelInfo {
		c.emitLogs(batchLogs{{level: TDLogLevelInfo, format: "Enqueue event data: %s", args: []interface{}{parseTime(jsonBytes)}}})
	}
	if ready || c.getCacheLength() > 0 {
		return c.Flush()
	}
	return nil
}

func (c *TDBatchConsumer) timerFlush() error { return c.innerFlush() }

func (c *TDBatchConsumer) Flush() error { return c.innerFlush() }

// Batch diagnostics are emitted only after releasing sender ownership.
type batchLogEntry struct {
	level  TDLogLevel
	format string
	args   []interface{}
}

type batchLogs []batchLogEntry

func (logs *batchLogs) add(level TDLogLevel, format string, args ...interface{}) {
	*logs = append(*logs, batchLogEntry{level: level, format: format, args: args})
}

func (c *TDBatchConsumer) innerFlush() error {
	var logs batchLogs
	err := c.flushWithLogs(&logs)
	return c.finishFlush(err, logs)
}

// Suppress nested/concurrent diagnostics for this consumer while a logger is
// running. This prevents log callbacks from producing an unbounded log loop;
// the underlying operations and their returned errors are unaffected.
func (c *TDBatchConsumer) emitLogs(logs batchLogs) {
	if len(logs) == 0 || !atomic.CompareAndSwapUint32(&c.emittingLogs, 0, 1) {
		return
	}
	defer atomic.StoreUint32(&c.emittingLogs, 0)
	for _, entry := range logs {
		tdLog(entry.level, entry.format, entry.args...)
	}
}

func (c *TDBatchConsumer) finishFlush(err error, logs batchLogs) error {
	if err == errBatchConsumerClosed {
		<-c.closeDone
		return c.closeErr
	}
	c.emitLogs(logs)
	return err
}

func (c *TDBatchConsumer) flushWithLogs(logs *batchLogs) error {
	c.sendGate <- struct{}{}
	defer func() { <-c.sendGate }()
	return c.flushWithGate(logs, false, false)
}

// Caller owns sendGate. Queue locks cover snapshot extraction/restoration only;
// compression, HTTP requests and retries never hold admission/cache locks.
func (c *TDBatchConsumer) flushWithGate(logs *batchLogs, all, closing bool) error {
	c.cacheMutex.Lock()
	c.bufferMutex.Lock()
	if !closing && atomic.LoadUint32(&c.closing) != 0 {
		c.bufferMutex.Unlock()
		c.cacheMutex.Unlock()
		return errBatchConsumerClosed
	}
	if len(c.buffer) > 0 && (all || len(c.cacheBuffer) == 0 || len(c.buffer) >= c.batchSize) {
		c.cacheBuffer = append(c.cacheBuffer, c.buffer)
		c.buffer = make([]json.RawMessage, 0, c.batchSize)
	}
	count := len(c.cacheBuffer)
	if !all && count > 1 {
		count = 1
	}
	pending := append([][]json.RawMessage(nil), c.cacheBuffer[:count]...)
	c.cacheBuffer = c.cacheBuffer[count:]
	c.bufferMutex.Unlock()
	c.cacheMutex.Unlock()
	var retained [][]json.RawMessage
	var firstErr error
	for _, batch := range pending {
		discard := false
		err := withPanicRecovery(func() error {
			var err error
			discard, err = c.uploadEvents(batch, logs)
			return err
		})
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if !discard {
			retained = append(retained, batch)
		}
		// This also reports recovered panics from a custom HTTP transport.
		if err != nil {
			logs.add(TDLogLevelError, "%v", err)
		}
	}
	c.cacheMutex.Lock()
	c.cacheBuffer = append(retained, c.cacheBuffer...)
	if len(c.cacheBuffer) > c.cacheCapacity {
		c.cacheBuffer = c.cacheBuffer[len(c.cacheBuffer)-c.cacheCapacity:]
	}
	c.cacheMutex.Unlock()
	return firstErr
}

// discard is true for every HTTP 200 response, preserving the deletion policy.
func (c *TDBatchConsumer) uploadEvents(buffer []json.RawMessage, logs *batchLogs) (discard bool, err error) {
	jsonBytes, err := json.Marshal(buffer)
	if err != nil {
		return false, err
	}
	params := parseTime(jsonBytes)
	for i := 0; i < 3; i++ {
		statusCode, code, err := c.send(params, len(buffer), logs)
		if statusCode == http.StatusOK {
			if err != nil {
				return true, err
			}
			switch code {
			case 0:
				logs.add(TDLogLevelInfo, "send success： %v", params)
				return true, nil
			case 1, -1:
				return true, errors.New("invalid data format")
			case -2:
				return true, errors.New("APP ID doesn't exist")
			case -3:
				return true, errors.New("invalid ip transmission")
			default:
				return true, errors.New("unknown error")
			}
		}
		if err != nil {
			return false, err
		}
		if i == 2 {
			return false, fmt.Errorf("network error, but err is nil. Status code is: %v", statusCode)
		}
	}
	return false, nil
}

// FlushAll attempts each currently buffered batch, continuing after failures.
// HTTP 200 batches are removed as before; undelivered batches remain cached.
func (c *TDBatchConsumer) FlushAll() error {
	var logs batchLogs
	err := c.flushAllWithLogs(&logs, false)
	return c.finishFlush(err, logs)
}

func (c *TDBatchConsumer) flushAllWithLogs(logs *batchLogs, closing bool) error {
	c.sendGate <- struct{}{}
	defer func() { <-c.sendGate }()
	return c.flushWithGate(logs, true, closing)
}

// Close stops admission and automatic flushing, then attempts every accepted
// batch. Repeated calls return the same result and do not resend failed batches.
func (c *TDBatchConsumer) Close() error {
	var logs batchLogs
	c.closeOnce.Do(func() {
		atomic.StoreUint32(&c.closing, 1)
		defer close(c.closeDone)
		if c.ticker != nil {
			c.ticker.Stop()
		}
		close(c.stopAutoFlush)
		c.closeErr = withPanicRecovery(func() error { return c.flushAllWithLogs(&logs, true) })
	})
	// Publish completion before invoking user loggers: they may reenter Close.
	// Do not join the timer here: Close can itself be called by a timer log callback.
	c.emitLogs(logs)
	return c.closeErr
}

func (c *TDBatchConsumer) IsStringent() bool {
	return false
}

func (c *TDBatchConsumer) send(data string, size int, logs *batchLogs) (statusCode int, code int, err error) {
	var encodedData string
	var compressType = "gzip"
	if c.compress {
		encodedData, err = encodeData(data)
	} else {
		encodedData = data
		compressType = "none"
	}
	if err != nil {
		return 0, 0, err
	}
	postData := bytes.NewBufferString(encodedData)

	var resp *http.Response
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", c.serverUrl, postData)
	if err != nil {
		return 0, 0, err
	}
	req.Header["appid"] = []string{c.appId}
	req.Header.Set("user-agent", "ta-go-sdk")
	req.Header.Set("version", SdkVersion)
	req.Header.Set("compress", compressType)
	req.Header["TA-Integration-Type"] = []string{LibName}
	req.Header["TA-Integration-Version"] = []string{SdkVersion}
	req.Header["TA-Integration-Count"] = []string{strconv.Itoa(size)}
	resp, err = c.HttpClient.Do(req)

	if err != nil {
		return 0, 0, err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logs.add(TDLogLevelError, "close response body error: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return resp.StatusCode, 1, fmt.Errorf("read batch response: %w", err)
		}
		var result struct {
			Code *int `json:"code"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			return resp.StatusCode, 1, fmt.Errorf("decode batch response: %w", err)
		}
		if result.Code == nil {
			return resp.StatusCode, 1, errors.New("invalid batch response: code is missing or null")
		}

		return resp.StatusCode, *result.Code, nil
	} else {
		return resp.StatusCode, -1, nil
	}
}

// Gzip
func encodeData(data string) (string, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	_, err := gw.Write([]byte(data))
	if err != nil {
		gw.Close()
		return "", err
	}
	gw.Close()

	return string(buf.Bytes()), nil
}

func (c *TDBatchConsumer) getBufferLength() int {
	c.bufferMutex.RLock()
	defer c.bufferMutex.RUnlock()
	return len(c.buffer)
}

func (c *TDBatchConsumer) getCacheLength() int {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()
	return len(c.cacheBuffer)
}
