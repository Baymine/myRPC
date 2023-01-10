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
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("Network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	geerpc.Accpt(l)
}

func main() {
	log.SetFlags(0)
	addr := make(chan string)
	go startServer(addr)
	client, _ := geerpc.Dial("tcp", <-addr)
	defer func() { _ = client.Close() }()

	time.Sleep(time.Second)
	// 使用等待组进行多个任务的同步，等待组可以保证在并发环境中完成指定数量的任务
	// 等待组内部拥有一个计数器，计数器的值可以通过方法调用实现计数器的增加和减少
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1) // wg +1
		go func(i int) {
			defer wg.Done() // wg -1
			args := fmt.Sprint("geerpc req %d", i)
			var reply string
			if err := client.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait() // 当等待组计数器不等于 0 时阻塞直到变 0。

}
