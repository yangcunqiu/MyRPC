package main

import (
	myrpc "MyRPC"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

func main() {
	log.SetFlags(0)
	addr := make(chan string)
	// 并发启动服务端
	go startServer(addr)

	client, _ := myrpc.Dial("tcp", <-addr)
	defer func() { client.Close() }()

	time.Sleep(time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			args := fmt.Sprintf("myrpc req %d", i)
			var reply string
			if err := client.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait()
}

func startServer(addr chan string) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	myrpc.Accept(l)
}
