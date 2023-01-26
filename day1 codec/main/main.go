package main

import (
	"context"
	"geerpc"
	"geerpc/registry"
	"geerpc/xclient"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Foo int

type Args struct{ Num1, Num2 int }

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}

func startRegistry(wg *sync.WaitGroup) {
	l, _ := net.Listen("tcp", ":9999")
	registry.HandleHTTP()
	wg.Done()
	_ = http.Serve(l, nil)
}

func startServer(registryAddr string, wg *sync.WaitGroup) {
	var foo Foo
	l, _ := net.Listen("tcp", ":0")
	server := geerpc.NewServer()
	_ = server.Register(&foo)
	// 定期向注册中心发送心跳保活。
	registry.Heartbeat(registryAddr, "tcp@"+l.Addr().String(), 0)
	wg.Done()
	server.Accept(l)
}

func foo(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

func call(registry string) {
	d := xclient.NewGeeRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() { _ = xc.Close() }()
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast(registry string) {
	d := xclient.NewGeeRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() { _ = xc.Close() }()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i * i})
			// expect 2 - 5 timeout
			ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			foo(xc, ctx, "broadcast", "Foo.Sleep", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func main() {
	log.SetFlags(0)
	registryAddr := "http://localhost:9999/_geerpc_/registry"
	var wg sync.WaitGroup
	wg.Add(1)
	go startRegistry(&wg)
	wg.Wait()

	time.Sleep(time.Second)
	wg.Add(2)
	go startServer(registryAddr, &wg)
	go startServer(registryAddr, &wg)
	wg.Wait()

	time.Sleep(time.Second)
	call(registryAddr)
	broadcast(registryAddr)
}

// day 6
//package main
//
//import (
//	"context"
//	"geerpc"
//	"geerpc/xclient"
//	"log"
//	"net"
//	"sync"
//	"time"
//)
//
//type Foo int
//
//type Args struct{ Num1, Num2 int }
//
//func (f Foo) Sum(args Args, reply *int) error {
//	*reply = args.Num1 + args.Num2
//	return nil
//}
//
//// Sleep 用于验证 XClient 的超时机制能否正常运作。
//func (f Foo) Sleep(args Args, reply *int) error {
//	time.Sleep(time.Second * time.Duration(args.Num1))
//	*reply = args.Num1 + args.Num2
//	return nil
//}
//
//func startServer(addrCh chan string) {
//	var foo Foo
//	l, _ := net.Listen("tcp", ":0")
//	server := geerpc.NewServer()
//	_ = server.Register(&foo)
//	addrCh <- l.Addr().String()
//	server.Accept(l)
//}
//
//// 封装一个方法 foo，便于在 Call 或 Broadcast 之后统一打印成功或失败的日志。
//func foo(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string, args *Args) {
//	var reply int
//	var err error
//	switch typ {
//	case "call":
//		err = xc.Call(ctx, serviceMethod, args, &reply)
//	case "broadcast":
//		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
//	}
//	if err != nil {
//		log.Printf("%s %s error: %v", typ, serviceMethod, err)
//	} else {
//		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
//	}
//}
//
//// call 调用单个服务实例
//func call(addr1, addr2 string) {
//	d := xclient.NewMultiServerDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})
//	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
//	defer func() { _ = xc.Close() }()
//	// send request & receive response
//	var wg sync.WaitGroup
//	for i := 0; i < 5; i++ {
//		wg.Add(1)
//		go func(i int) {
//			defer wg.Done()
//			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
//		}(i)
//	}
//	wg.Wait()
//}
//
//// broadcast 调用所有服务实例
//func broadcast(addr1, addr2 string) {
//	d := xclient.NewMultiServerDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})
//	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
//	defer func() { _ = xc.Close() }()
//	var wg sync.WaitGroup
//	for i := 0; i < 5; i++ {
//		wg.Add(1)
//		go func(i int) {
//			defer wg.Done()
//			foo(xc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i * i})
//			// expect 2 - 5 timeout
//			ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
//			foo(xc, ctx, "broadcast", "Foo.Sleep", &Args{Num1: i, Num2: i * i})
//		}(i)
//	}
//	wg.Wait()
//}
//
//func main() {
//	log.SetFlags(0)
//	ch1 := make(chan string)
//	ch2 := make(chan string)
//	// start two servers
//	go startServer(ch1)
//	go startServer(ch2)
//
//	addr1 := <-ch1
//	addr2 := <-ch2
//
//	time.Sleep(time.Second)
//	call(addr1, addr2)
//	broadcast(addr1, addr2)
//}

/**
day 5
*/
//package main
//
//import (
//	"context"
//	"geerpc"
//	"log"
//	"net"
//	"net/http"
//	"sync"
//	"time"
//)
//
//type Foo int
//
//type Args struct{ Num1, Num2 int }
//
//func (f Foo) Sum(args Args, reply *int) error {
//	*reply = args.Num1 + args.Num2
//	return nil
//}
//
//func startServer(addrCh chan string) {
//	var foo Foo
//	l, _ := net.Listen("tcp", ":9999")
//	_ = geerpc.Register(&foo)
//	geerpc.HandleHTTP()
//	addrCh <- l.Addr().String()
//	_ = http.Serve(l, nil)
//}
//
//func call(addrCh chan string) {
//	client, _ := geerpc.DialHTTP("tcp", <-addrCh)
//	defer func() { _ = client.Close() }()
//
//	time.Sleep(time.Second)
//	// send request & receive response
//	var wg sync.WaitGroup
//	for i := 0; i < 5; i++ {
//		wg.Add(1)
//		go func(i int) {
//			defer wg.Done()
//			args := &Args{Num1: i, Num2: i * i}
//			var reply int
//			if err := client.Call(context.Background(), "Foo.Sum", args, &reply); err != nil {
//				log.Fatal("call Foo.Sum error:", err)
//			}
//			// 在输出前面加上了时间信息
//			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
//		}(i)
//	}
//	wg.Wait()
//}
//
//func main() {
//	log.SetFlags(0) // log 添加时间，1：添加日期， 2：添加具体时间
//	ch := make(chan string)
//	go call(ch)
//	startServer(ch)
//}

/*day 4*/
//package main
//
//import (
//	"geerpc"
//	"log"
//	"net"
//	"sync"
//	"time"
//)
//
//// 第一步，定义结构体 Foo 和方法 Sum
//
//type Foo int
//
//type Args struct{ Num1, Num2 int }
//
//func (f Foo) Sum(args Args, reply *int) error {
//	*reply = args.Num1 + args.Num2
//	return nil
//}
//
//// 第二步，注册 Foo 到 Server 中，并启动 RPC 服务
//
//func startServer(addr chan string) {
//	var foo Foo
//	if err := geerpc.Register(&foo); err != nil {
//		log.Fatal("register error:", err)
//	}
//	// pick a free port
//	l, err := net.Listen("tcp", ":0")
//	if err != nil {
//		log.Fatal("network error:", err)
//	}
//	log.Println("start rpc server on", l.Addr())
//	addr <- l.Addr().String()
//	geerpc.Accept(l)
//}
//
//// 第三步，构造参数，发送 RPC 请求，并打印结果。
//
//func main() {
//	log.SetFlags(0)
//	addr := make(chan string)
//	go startServer(addr)
//	client, _ := geerpc.Dial("tcp", <-addr)
//	defer func() { _ = client.Close() }()
//
//	time.Sleep(time.Second)
//	// send request & receive response
//	var wg sync.WaitGroup
//	for i := 0; i < 5; i++ {
//		wg.Add(1)
//		go func(i int) {
//			defer wg.Done()
//			args := &Args{Num1: i, Num2: i * i}
//			var reply int
//			if err := client.Call("Foo.Sum", args, &reply); err != nil {
//				log.Fatal("call Foo.Sum error:", err)
//			}
//			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
//		}(i)
//	}
//	wg.Wait()
//}

// Day1~2
//package main
//
//import (
//	"fmt"
//	"geerpc"
//	"log"
//	"net"
//	"sync"
//	"time"
//)
//
//func startServer(addr chan string) {
//	// Listen(network, address string) (Listener, error)
//	l, err := net.Listen("tcp", ":0") // ":0" : a port number is automatically chosen
//	if err != nil {
//		log.Fatal("network error:", err)
//	}
//
//	log.Println("start rpc server on", l.Addr())
//	addr <- l.Addr().String() // 向管道发送数据
//	geerpc.Accept(l)
//}
//
//func main() {
//	log.SetFlags(0)           // 不加会阻塞
//	addr := make(chan string) // 从协程中获取地址信息
//	go startServer(addr)
//
//	//conn, _ := net.Dial("tcp", <-addr)
//	client, _ := geerpc.Dial("tcp", <-addr)
//	defer func() { _ = client.Close() }()
//
//	time.Sleep(time.Second)
//
//	var wg sync.WaitGroup
//	for i := 0; i < 5; i++ {
//		wg.Add(1)
//		go func(i int) {
//			defer wg.Done()
//
//			args := fmt.Sprintf("geerpc req %d", i)
//			var reply string
//			if err := client.Call("Foo.Sum", args, &reply); err != nil {
//				log.Fatal("call Foo.Sum error:", err)
//			}
//			log.Println("reply:", reply)
//		}(i)
//	}
//	wg.Wait()
//
//}
