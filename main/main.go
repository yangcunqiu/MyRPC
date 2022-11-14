package main

import (
	myrpc "MyRPC"
	"MyRPC/codec"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	addr := make(chan string)
	// 并发启动服务端
	go startServer(addr)

	// 同步channel 保证服务器监听成功, 客户端再发起请求
	coon, _ := net.Dial("tcp", <-addr)
	defer func() {
		coon.Close()
	}()

	time.Sleep(time.Second)
	_ = json.NewEncoder(coon).Encode(myrpc.DefaultOption)
	cc := codec.NewGobCodec(coon)
	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}
		cc.Write(h, fmt.Sprintf("myrpc req %d", h.Seq))
		cc.ReadHeader(h)
		var reply string
		cc.ReadBody(&reply)
		log.Println("reply:", reply)
	}
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
