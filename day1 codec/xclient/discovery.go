package xclient

/**
负载均衡的前提是有多个服务实例，那我们首先实现一个最基础的服务发现模块 Discovery。为了与通信部分解耦
这部分的代码统一放置在 xclient 子目录下
*/

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

type SelectMode int // 代表不同类型的负载均衡策略（GeeRPC 仅实现 Random 和 RoundRobin 两种策略。）

const (
	RandomSelect     SelectMode = iota // select randomly
	RoundRobinSelect                   // select using Robbin algorithm
)

// Discovery 包含了服务发现所需要的最基本的接口
type Discovery interface {
	Refresh() error                      // refresh from remote registry 从注册中心更新服务列表
	Update(servers []string) error       // 手动更新服务列表
	Get(mode SelectMode) (string, error) // 根据负载均衡策略，选择一个服务实例
	GetAll() ([]string, error)           // 返回所有的服务实例
}

var _ Discovery = (*MultiServersDiscovery)(nil)

// MultiServersDiscovery is a discovery for multi servers without a registry center
// user provides the server addresses explicitly instead
// 一个不需要注册中心，服务列表由手工维护的服务发现的结构体
type MultiServersDiscovery struct {
	r       *rand.Rand   // generate random number 初始化时使用时间戳设定随机数种子，避免每次产生相同的随机数序列。
	mu      sync.RWMutex // protect following
	servers []string     // 存储多个服务实例
	// 记录 Round Robin 算法已经轮询到的位置，为了避免每次从 0 开始，初始化时随机设定一个值。
	index int // record the selected position for robin algorithm
}

// Refresh doesn't make sense for MultiServersDiscovery, so ignore it
func (d *MultiServersDiscovery) Refresh() error {
	return nil
}

// Update the servers of discovery dynamically if needed
func (d *MultiServersDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	return nil
}

// Get a server according to mode
// 传入负载均衡的策略，执行相对应的负载均衡（随机和roundRobin）
func (d *MultiServersDiscovery) Get(mode SelectMode) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.servers)
	if n == 0 {
		return "", errors.New("rpc discovery: no available servers")
	}
	switch mode {
	case RandomSelect:
		return d.servers[d.r.Intn(n)], nil
	case RoundRobinSelect:
		s := d.servers[d.index%n] // servers could be updated, so mode n to ensure safet	y
		d.index = (d.index + 1) % n
		return s, nil
	default:
		return "", errors.New("rpc discovery: not supported select mode")
	}
}

// GetAll returns all servers in discovery
// 就是复制一份MultiServersDiscovery的服务列表
func (d *MultiServersDiscovery) GetAll() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	// return a copy of d.servers
	servers := make([]string, len(d.servers), len(d.servers))
	copy(servers, d.servers)
	return servers, nil
}

// NewMultiServerDiscovery creates a MultiServersDiscovery instance
func NewMultiServerDiscovery(servers []string) *MultiServersDiscovery {
	d := &MultiServersDiscovery{
		servers: servers,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1) // Round Robin 轮询到的位置
	return d
}
