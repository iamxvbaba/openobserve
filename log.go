package openobserve

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

type basicAuth struct {
	username string
	password string
}

type Option func(*OpenObLog)

type OpenObLog struct {
	*sync.Mutex
	ctx            context.Context
	httpc          *http.Client
	addr           string
	c              chan any
	list           []any
	basicAuth      *basicAuth
	authorization  *string
	fullSize       int // 达到此数量时不再等waitTime到期,直接写出
	waitTime       time.Duration
	requestTimeout time.Duration

	compress      bool // 请求报文进行gzip压缩传输
	compressLevel int  // 请求报文gzip压缩级别，-1~9，0：不压缩，9：最高压缩，-1：默认压缩

	everyDayCut bool
	indexName   string
}

// WithBasicAuth 使用账号密码
func WithBasicAuth(username, password string) Option {
	return func(log *OpenObLog) {
		log.basicAuth = &basicAuth{
			username: username, password: password,
		}
	}
}

// WithAuthorization 使用token
func WithAuthorization(token string) Option {
	return func(log *OpenObLog) {
		token = "Basic " + token
		log.authorization = &token
	}
}

// WithFullSize 个数达到此配置马上请求
func WithFullSize(fullSize int) Option {
	return func(log *OpenObLog) {
		log.fullSize = fullSize
		if log.c != nil {
			close(log.c)
		}
		log.c = make(chan any, fullSize+16)
	}
}

// WithWaitTime 每过此配置的发起请求
func WithWaitTime(td time.Duration) Option {
	return func(log *OpenObLog) {
		log.waitTime = td
	}
}

// WithRequestTimeout http 请求超时
func WithRequestTimeout(tm time.Duration) Option {
	return func(log *OpenObLog) {
		log.requestTimeout = tm
	}
}

// WithIndexName name索引名,cut是否按天切割
func WithIndexName(name string, cut bool) Option {
	return func(log *OpenObLog) {
		log.everyDayCut = cut
		log.indexName = name
	}
}
func buildLastName(name string) string {
	return fmt.Sprintf("%s_%s", name,
		time.Now().Format("20060102"))
}

// WithCompress 请求报文gzip压缩
func WithCompress(compress bool, compressLevel ...int) Option {
	// 如果没有传递 compressLevel 参数，则使用默认值 -1
	var level int
	if len(compressLevel) > 0 {
		level = compressLevel[0]

		// 验证 compressLevel 是否在有效范围 -1 到 9 之间
		if level < -1 || level > 9 {
			log.Println("Invalid compress level:", level, "Valid range is -1 to 9")
			level = -1 // 设置为默认值
		}
	} else {
		level = -1 // 默认压缩级别
	}

	return func(log *OpenObLog) {
		log.compress = compress
		log.compressLevel = level
	}
}

func New(ctx context.Context, addr string, options ...Option) *OpenObLog {
	l := &OpenObLog{
		Mutex:          &sync.Mutex{},
		ctx:            ctx,
		addr:           addr,
		fullSize:       16,
		waitTime:       0,
		requestTimeout: time.Second * 10,
		indexName:      "open",
	}
	for _, option := range options {
		option(l)
	}
	l.httpc = &http.Client{}
	l.httpc.Timeout = l.requestTimeout
	l.list = make([]any, 0, l.fullSize)
	l.c = make(chan any, l.fullSize*2) // 管道长度是缓冲区2倍，增加冗余，防止刷缓冲区的时候，发送协程被阻塞

	l.run()
	return l
}
func (l *OpenObLog) run() {
	go func() {
		for {
			select {
			case val := <-l.c:
				l.Lock()
				l.list = append(l.list, val)
				l.Unlock()
				if len(l.list) >= l.fullSize {
					l.sendList()
				}
			case <-l.ctx.Done():
				l.sendList()
				return
			}

		}
	}()
	if l.waitTime > 0 {
		go func() {
			ticker := time.NewTicker(l.waitTime)
			for {
				select {
				case <-ticker.C:
					l.sendList()
				case <-l.ctx.Done():
					l.sendList()
					return
				}
			}
		}()
	}
}

func (l *OpenObLog) request(obj any) bool {
	// 首先将日志对象序列化为 JSON 格式
	data, err := json.Marshal(obj)
	if err != nil {
		log.Println("OpenObLog.request json.Marshal:", err)
		return false
	}

	// 发送缓冲区
	var buf bytes.Buffer

	// 预设一个标志，判断请求报文是否进行压缩
	bReqCompress := false

	// 如果启用压缩且压缩级别有效，进行压缩
	if l.compress {
		gz, err := gzip.NewWriterLevel(&buf, l.compressLevel)
		if err != nil {
			log.Println("OpenObLog.request gzip.NewWriterLevel:", err)
		} else {
			// 压缩数据
			if _, err = gz.Write(data); err != nil {
				log.Println("OpenObLog.request gzip compression:", err)
				buf.Reset()
			}
			gz.Close()
			bReqCompress = true
		}
	}

	if !bReqCompress {
		// 如果没有启用压缩，直接发送原始数据
		buf.Write(data)
	}

	// 根据每日报表切换索引名称
	indexName := l.indexName
	if l.everyDayCut {
		indexName = buildLastName(l.indexName)
	}

	// 创建 HTTP 请求，使用 buf 作为请求体
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/default/%s/_json", l.addr, indexName),
		&buf)
	if err != nil {
		log.Println("OpenObLog.request NewRequest:", err)
		return false
	}

	req.Header.Set("Content-Type", "application/json")
	if l.authorization != nil {
		req.Header.Set("Authorization", *l.authorization)
	}
	if l.basicAuth != nil {
		req.SetBasicAuth(l.basicAuth.username, l.basicAuth.password)
	}

	// 设置 Content-Encoding 为 gzip（如果启用了压缩）
	if bReqCompress {
		req.Header.Set("Content-Encoding", "gzip")
	}

	// 发送 HTTP 请求
	response, err := l.httpc.Do(req)
	if err != nil {
		log.Println("OpenObLog.request DoRequest:", err)
		return false
	}
	defer response.Body.Close()

	// 检查响应状态
	if response.StatusCode == http.StatusOK {
		return true
	}

	body, _ := io.ReadAll(response.Body)
	log.Println("OpenObLog.request request error:", string(body))
	return false
}

func (l *OpenObLog) sendList() {
	l.Lock()
	defer l.Unlock()
	if len(l.list) == 0 {
		return
	}
	if l.request(l.list) || len(l.list) > l.fullSize*10 {
		l.list = l.list[:0] // 如果多次发送失败，应及时清除缓存，防止积攒在内存造成业务异常 TODO：增加可选参数
	}
}

// SendSync 同步发送
func (l *OpenObLog) SendSync(data any) bool {
	if data == nil {
		return
	}
	return l.request(data)
}

// Send 异步发送
func (l *OpenObLog) Send(data any) {
	if data == nil {
		return
	}
	select {
	case l.c <- data:
	default:
		log.Println("OpenObserve send error by full size.")
	}
}
