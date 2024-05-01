package openobserve

import (
	"bytes"
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
func New(ctx context.Context, addr string, options ...Option) *OpenObLog {
	l := &OpenObLog{
		Mutex:          &sync.Mutex{},
		ctx:            ctx,
		addr:           addr,
		c:              make(chan any, 16),
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

	l.run()
	return l
}
func (l *OpenObLog) run() {
	go func() {
		for {
			select {
			case val := <-l.c:
				if l.waitTime <= 0 {
					l.send(val)
					continue
				}
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
	data, err := json.Marshal(obj)
	if err != nil {
		log.Println("OpenObLog.request json.Marshal:", err)
		return false
	}
	indexName := l.indexName
	if l.everyDayCut {
		indexName = buildLastName(l.indexName)
	}
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/default/%s/_json", l.addr, indexName),
		bytes.NewReader(data))
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
	response, err := l.httpc.Do(req)
	if err != nil {
		log.Println("OpenObLog.request DoRequest:", err)
		return false
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		return true
	}
	body, _ := io.ReadAll(response.Body)
	log.Println("OpenObLog.request request error:", string(body))
	return false
}
func (l *OpenObLog) send(data any) {
	if data == nil {
		return
	}
	l.request(data)
}
func (l *OpenObLog) sendList() {
	l.Lock()
	defer l.Unlock()
	if len(l.list) == 0 {
		return
	}
	if l.request(l.list) {
		l.list = l.list[:0]
	}
}

// SendSync 同步发送
func (l *OpenObLog) SendSync(data any) bool {
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
