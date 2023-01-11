package geerpc

import (
	"encoding/json"
	"fmt"
	"geerpc/codec"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const MagicNumber = 0xbef5c

// Option 对于 GeeRPC 来说，目前需要协商的唯一一项内容是消息的编解码方式
type Option struct {
	MagicNumber int        // MagicNumber marks this a geerpc request
	CodecType   codec.Type // 编码方式
}

// DefaultOption 为了实现上简单，Option采用JSON编码，后续的Header和body采用选择的编码方式
var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

// Server Represent an RPC server
type Server struct{}

func NewServer() *Server {
	return &Server{} // 最后的{}标识对结构体的初始化
}

var DefaultServer = NewServer() // ()表示对函数的调用

// Server结构体的成员函数

// ServeConn 利用连接对客户端的选项信息进行解码，并返回选项中的解码函数
// runs the server on a single connection. blocks, serving the connection until the client hangs up.
func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() { _ = conn.Close() }()

	var opt Option
	// conn: source(a Reader); &opt: stored the request information
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error: ", err)
		return
	}

	if opt.MagicNumber != MagicNumber {
		log.Printf("rpc server: invalid magic number %x\n", opt.MagicNumber)
		return
	}
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", &opt.CodecType)
		return
	}

	// param: 编码类的构造函数，构造编码类需要利用连接初始化
	server.serveCodec(f(conn))
}

// invalidRequest is a placeholder for response argv when error occurs
var invalidRequest = struct{}{}

func (server *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex) // 保证一个完整的回复
	wg := new(sync.WaitGroup)  // 等待所有的请求都被处理
	for {                      // 允许接收多个请求
		req, err := server.readRequest(cc) // 读取请求
		if err != nil {                    // 错误：连接关闭，接收到的报文有问题
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, invalidRequest, sending) // 回复请求
			continue
		}
		wg.Add(1)                                     // 接收并处理请求
		go server.handleRequest(cc, req, sending, wg) // 处理请求
	}
	wg.Wait()
	_ = cc.Close()
}

// request stores all information of a call
type request struct {
	h            *codec.Header // header of request
	argv, replyv reflect.Value // argv and replyv of request
}

// 返回解码后的报头信息
func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
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

// 从编码类中获取请求信息（编码类中包含连接），放到request结构体中
func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}

	req := &request{h: h} // 结构体request的赋值
	// TODO: just suppose it's string
	req.argv = reflect.New(reflect.TypeOf(""))

	// 读取连接中的参数信息（传输主体就是参数列表）, 并存放到req.argv.Interface()中
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server: read argv err:", err)
	}
	return req, nil
}

// 将报头和主体发送给客户端（使用的是bufio.Writer）
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

// Accept for 循环等待 socket 连接建立，并开启子协程处理，处理过程交给了 ServerConn 方法。
func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error: ", err)
			return
		}
		go server.ServeConn(conn) // 主线程监听，子线程处理
	}
}

// Accept 接收并处理连接
func Accept(lis net.Listener) { DefaultServer.Accept(lis) }
