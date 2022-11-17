package main

import (
	myrpc "MyRPC"
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Foo int

type Args struct {
	Num1, Num2 int
}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func main() {
	log.SetFlags(0)
	ch := make(chan string)
	// 并发启动服务端
	go call(ch)
	startServer(ch)
}

func startServer(addr chan string) {
	var foo Foo
	l, _ := net.Listen("tcp", ":9999")
	_ = myrpc.Register(&foo)
	myrpc.HandleHTTP()
	addr <- l.Addr().String()
	http.Serve(l, nil)
}

func call(addrCh chan string) {
	client, _ := myrpc.DialHTTP("tcp", <-addrCh)
	defer func() {
		client.Close()
	}()

	time.Sleep(time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{Num1: i, Num2: i * i}
			var reply int
			if err := client.Call(context.Background(), "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			log.Printf("%d + %d = %d:", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}
