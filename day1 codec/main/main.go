package main

import (
	"context"
	"geerpc"
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

func startServer(addrCh chan string) {
	var foo Foo
	l, _ := net.Listen("tcp", ":9999")
	_ = geerpc.Register(&foo)
	geerpc.HandleHTTP()
	addrCh <- l.Addr().String()
	_ = http.Serve(l, nil)
}

func call(addrCh chan string) {
	client, _ := geerpc.DialHTTP("tcp", <-addrCh)
	defer func() { _ = client.Close() }()

	time.Sleep(time.Second)
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{Num1: i, Num2: i * i}
			var reply int
			if err := client.Call(context.Background(), "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			// 在输出前面加上了时间信息
			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}

func main() {
	log.SetFlags(0) // log 添加时间，1：添加日期， 2：添加具体时间
	ch := make(chan string)
	go call(ch)
	startServer(ch)
}

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
