package openobserve

import (
	"context"
	"testing"
	"time"
)

func TestOpenObLog_SendSyncWithAuthorizer(t *testing.T) {
	obLog := New(context.TODO(), "http://localhost:5080",
		WithAuthorization("OTQ0ODA4NjY2QHFxLmNvbTpCMWxQb0ZYQTJ5UEFnZ05B"))
	result := obLog.SendSync(map[string]any{
		"_timestamp": time.Now().UnixMicro(),
		"name":       "xuhui",
		"message":    "你好",
		"log":        "ip-10-2-56-221.us-east-2.compute.internal",
	})
	t.Log(result)
}
func TestOpenObLog_SendWithIndexName(t *testing.T) {
	obLog := New(context.TODO(), "http://localhost:5080",
		WithAuthorization("OTQ0ODA4NjY2QHFxLmNvbTpCMWxQb0ZYQTJ5UEFnZ05B"),
		WithIndexName("xuhui", true),
		WithWaitTime(time.Second*3))

	for i := 0; i < 15; i++ {
		obLog.Send(map[string]any{
			"_timestamp": time.Now().UnixMicro(),
			"name":       "xuhui",
			"message":    "你好",
			"log":        "ip-10-2-56-221.us-east-2.compute.internal",
			"i":          i,
		})
		time.Sleep(time.Millisecond * 100)
	}

	time.Sleep(time.Second * 10)
}
func TestOpenObLog_SendWithFullSize(t *testing.T) {
	obLog := New(context.TODO(), "http://localhost:5080",
		WithAuthorization("OTQ0ODA4NjY2QHFxLmNvbTpCMWxQb0ZYQTJ5UEFnZ05B"),
		WithIndexName("xuhui", true),
		WithFullSize(2),
		WithWaitTime(time.Second*6))

	for i := 0; i < 28; i++ {
		obLog.Send(map[string]any{
			"_timestamp": time.Now().UnixMicro(),
			"name":       "xuhui",
			"message":    "你好",
			"log":        "ip-10-2-56-221.us-east-2.compute.internal",
			"i":          i,
		})
		time.Sleep(time.Second)
	}

	time.Sleep(time.Second * 10)
}
