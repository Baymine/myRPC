package main

import (
	"fmt"
	"geerpc"
	"log"
	"net"
	"sync"
	"time"
)

func startServer(addr chan string) {
	// Listen(network, address string) (Listener, error)
	l, err := net.Listen("tcp", ":0") // ":0" : a port number is automatically chosen
	if err != nil {
		log.Fatal("network error:", err)
	}

	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String() // 向管道发送数据
	geerpc.Accept(l)
}

func main() {
	log.SetFlags(0)           // 不加会阻塞
	addr := make(chan string) // 从协程中获取地址信息
	go startServer(addr)

	//conn, _ := net.Dial("tcp", <-addr)
	client, _ := geerpc.Dial("tcp", <-addr)
	defer func() { _ = client.Close() }()

	time.Sleep(time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			args := fmt.Sprintf("geerpc req %d", i)
			var reply string
			if err := client.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait()

}
