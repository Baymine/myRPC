package main

import (
	"encoding/json"
	"fmt"
	"geerpc"
	"geerpc/codec"
	"log"
	"net"
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
	addr := make(chan string) // 从协程中获取地址信息
	go startServer(addr)      // 子线程去处理连接

	conn, _ := net.Dial("tpc", <-addr)
	defer func() {
		_ = conn.Close()
	}()

	time.Sleep(time.Second)
	// 选项：编码方法
	_ = json.NewEncoder(conn).Encode(geerpc.DefaultOption)
	cc := codec.NewGobCodec(conn)

	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}
		_ = cc.Write(h, fmt.Sprintf("geerpc req %d", h.Seq)) // send
		_ = cc.ReadHeader(h)                                 // receive header

		var reply string
		_ = cc.ReadBody(&reply) // receive body
		log.Println("reply:", reply)
	}
}
