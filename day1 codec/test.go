package geerpc

import (
	"context"
	"fmt"
	"time"
)

func main() {
	// parent context
	ctx, cancel := context.WithCancel(context.Background())
	go watch1(ctx)

	// child context
	valueCtx, _ := context.WithCancel(ctx)
	go watch2(valueCtx)

	fmt.Println("Wait for 3 sec, time=", time.Now().Unix())
	time.Sleep(3 * time.Second)

	fmt.Println("wait 3 sec, calling cancel function")
	cancel()

	time.Sleep(5 * time.Second)
	fmt.Println("Final exit, time=", time.Now().Unix())
}

func watch1(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Signal receive, father context exit, time=", time.Now().Unix())
			return
		default:
			fmt.Println("father context watching, time=", time.Now().Unix())
			time.Sleep(1 * time.Second)
		}
	}
}

func watch2(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("signal receive, child context goroutine exit, time=", time.Now().Unix())
			return
		default:
			fmt.Println("child context goroutine watching, time=", time.Now().Unix())
			time.Sleep(1 * time.Second)
		}
	}
}
