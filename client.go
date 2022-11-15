package myrpc

import (
	"MyRPC/codec"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// Call 一次rpc调用所需要的信息
type Call struct {
	Seq           uint64
	ServiceMethod string
	Args          interface{}
	Reply         interface{}
	Error         error
	Done          chan *Call
}

func (call *Call) done() {
	call.Done <- call
}

// Client 客户端
type Client struct {
	cc       codec.Codec
	opt      *Option
	sending  sync.Mutex
	header   codec.Header
	mu       sync.Mutex
	seq      uint64           // 请求编号
	pending  map[uint64]*Call // 存储未处理完的请求，键是编号，值是 Call 实例
	closing  bool
	shutdown bool
}

var _ io.Closer = (*Client)(nil)

var ErrShutdown = errors.New("connection is shut down")

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return client.cc.Close()
	}
	client.closing = true
	return client.cc.Close()
}

func (client *Client) isAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.shutdown && !client.closing
}

// registerCall 向client注册call
func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	client.mu.Unlock()
	// 客户端关闭, 直接返回
	if client.closing || client.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

// removeCall 根据编号seq移除call, 返回被移除的call
func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

// terminateCalls 终止call
func (client *Client) terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Lock()
	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		// 通知每个call
		call.done()
	}
}

// receive 接收server响应
func (client *Client) receive() {
	var err error
	// 循环接收, 直到出错
	for err == nil {
		var h codec.Header
		if err = client.cc.ReadHeader(&h); err != nil {
			// 读取header出错
			break
		}
		// 将call从client未处理请求map中移除
		call := client.removeCall(h.Seq)
		switch {
		case call == nil:
			// 请求没有发送完整，或者因为其他原因被取消，但是服务端仍旧处理了
			err = client.cc.ReadBody(nil)
		case h.Error != "":
			// 服务端处理出错
			call.Error = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default:
			// 正常
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	// 接收过程client或server出错, 终止调用
	client.terminateCalls(err)
}

// NewClient 创建client
func NewClient(coon net.Conn, opt *Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}
	// 协议交换
	if err := json.NewEncoder(coon).Encode(opt); err != nil {
		log.Println("rpc client: options error: ", err)
		coon.Close()
		return nil, err
	}
	return newClientCodec(f(coon), opt), nil
}

func newClientCodec(cc codec.Codec, opt *Option) *Client {
	client := &Client{
		seq:     1,
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	// 接收请求
	go client.receive()
	return client
}

// parseOptions 解析Option
func parseOptions(opts ...*Option) (*Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = DefaultOption.CodecType
	}
	return opt, nil
}

func Dial(network, address string, opts ...*Option) (client *Client, err error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	defer func() {
		if client == nil {
			conn.Close()
		}
	}()
	return NewClient(conn, opt)
}

// send 向server发送消息
func (client *Client) send(call *Call) {
	client.sending.Lock()
	defer client.sending.Unlock()

	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// 构建header
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	// 编码并发送请求
	if err = client.cc.Write(&client.header, call.Args); err != nil {
		call := client.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(call)
	return call
}

func (client *Client) Call(serviceMethod string, args, reply interface{}) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}
