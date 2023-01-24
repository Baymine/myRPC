package xclient

import (
	"context"
	. "geerpc"
	"io"
	"reflect"
	"sync"
)

type XClient struct {
	d       Discovery
	mode    SelectMode
	opt     *Option
	mu      sync.Mutex         // protect following
	clients map[string]*Client // 保存创建成功的Client实例
}

var _ io.Closer = (*XClient)(nil)

// NewXClient
//
//	@Description: XClient 的构造函数
//	@param d：服务发现实例 Discovery
//	@param mode：负载均衡模式 SelectMode
//	@param opt：协议选项 Option
//	@return *XClient： 新建的XClient实例
func NewXClient(d Discovery, mode SelectMode, opt *Option) *XClient {
	return &XClient{d: d, mode: mode, opt: opt, clients: make(map[string]*Client)}
}

// Close 遍历client数组，逐个关闭其中的client，并删除键值对
func (xc *XClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	for key, client := range xc.clients {
		// I have no idea how to deal with error, just ignore it.
		_ = client.Close()
		delete(xc.clients, key)
	}
	return nil
}

// dial
//
//	@Description: 对Client进行复用。先检查是否可用client，有，直接返回；没有，创建
//	@receiver xc
//	@param rpcAddr：服务地址
//	@return *Client： 缓存中的或者创建的Client实例
//	@return error
func (xc *XClient) dial(rpcAddr string) (*Client, error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	client, ok := xc.clients[rpcAddr]
	if ok && !client.IsAvailable() { // 检查是否是可用状态
		_ = client.Close()
		delete(xc.clients, rpcAddr)
		client = nil
	}
	if client == nil { // 没有返回缓存的client，需要重新创建，后缓存
		var err error
		client, err = XDial(rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		xc.clients[rpcAddr] = client
	}
	return client, nil
}

func (xc *XClient) call(rpcAddr string, ctx context.Context, serviceMethod string, args, reply interface{}) error {
	client, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return client.Call(ctx, serviceMethod, args, reply)
}

// Call invokes the named function, waits for it to complete,
// and returns its error status.
// xc will choose a proper server.
func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	rpcAddr, err := xc.d.Get(xc.mode) // 通过负载均衡来选择服务地址
	if err != nil {
		return err
	}
	return xc.call(rpcAddr, ctx, serviceMethod, args, reply)
}

// Broadcast invokes the named function for every server registered in discovery
// 将请求广播到所有的服务实例，如果任意一个实例发生错误，则返回其中一个错误；
// 如果调用成功，则返回其中一个的结果。有以下几点需要注意：
//
// 为了提升性能，请求是并发的。
// 并发情况下需要使用互斥锁保证 error 和 reply 能被正确赋值。
// 借助 context.WithCancel 确保有错误发生时，快速失败。
func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	servers, err := xc.d.GetAll()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	var mu sync.Mutex // protect e and replyDone
	var e error
	replyDone := reply == nil              // if reply is nil, don't need to set value
	ctx, cancel := context.WithCancel(ctx) // 接受一个 Context 并返回其子Context和取消函数cancel
	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var clonedReply interface{}
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := xc.call(rpcAddr, ctx, serviceMethod, args, clonedReply)
			mu.Lock()
			if err != nil && e == nil {
				e = err
				// 外面调用 cancel 函数，即会往子Context的Done通道发送消息
				cancel() // if any call failed, cancel unfinished calls
			}
			if err == nil && !replyDone {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	return e
}
