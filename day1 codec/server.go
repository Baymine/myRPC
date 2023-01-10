package geerpc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const MagicNumber = 0xbef5c

// 对于 GeeRPC 来说，目前需要协商的唯一一项内容是消息的编解码方式
type Option struct {
	MagicNumber int        // MagicNumber marks this's a geerpc request
	CodecType   codec.Type // 编码方式
}

// 为了实现上简单，Option采用JSON编码，后续的Header和body采用选择的编码方式
var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

// Represent a RPC server
type Server struct{}

func NewServer() *Server {
	return &Server{} // 最后的{}标识对结构体的初始化
}

var DefaltServer = NewServer() // ()表示对函数的调用

// Server结构体的成员函数

// ServeConn runs the server on a single connection.
// ServeConn blocks, serving the connection until the client hangs up.
func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() { _ = conn.Close() }()

	var opt Option
	// conn: source(a Reader); &opt: stored the request information
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error: ", err)
		return
	}

	if opt.MagicNumber != MagicNumber {
		log.Println("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", &opt.CodecType)
		return
	}
	server.serveCodec(f(conn)) // param: 编码类的构造函数
}

// invalidRequest is a placeholder for response argv when error occurs
var invalidReqest = struct{}{}

func (server *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex) // 保证一个完整的回复
	wg := new(sync.WaitGroup)  // 等待所有的请求都被处理
	for {                      // 允许接收多个请求
		req, err := server.readRquest(cc) // 读取请求
		if err != nil {                   // 错误：连接关闭，接收到的报文有问题
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, invalidReqest, sending) // 回复请求
			continue
		}
		wg.Add(1)
		go server.handleRequest(cc, req, sending, wg) // 处理请求
	}
	wg.Wait()
	_ = cc.Close()
}

type request struct {
	h            *codec.Header
	argv, replyv reflect.Value
}

// 返回解码后的报头信息
func (server *Server) readRquestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header

	// cc.ReadHeader(&h)： 将从conn中解码到的信息放到h中
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (server *Server) readRquest(cc codec.Codec) (*request, error) {
	h, err := server.readRquestHeader(cc)
	if err != nil {
		return nil, err
	}

	req := &request{h: h} // 结构体request的赋值
	// TODO: just suppose it's string
	req.argv = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server: read argv err:", err)
	}
	return req, nil
}

func (server *Server) sendResponse(cc codec.Codec,
	h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()

	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	// TODO: should call registered rpc methods to get the right replyv
	// day 1, just print argv and send a hello message
	defer wg.Done()
	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.h.Seq))
	server.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}

func (server *Server) Accept(lis net.Listener) {
	for { // for 循环等待 socket 连接建立，并开启子协程处理，处理过程交给了 ServerConn 方法。
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error: ", err)
			return
		}
		go server.ServeConn(conn)
	}
}

func Accept(lis net.Listener) { DefaltServer.Accept(lis) }
