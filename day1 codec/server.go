package geerpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
)

const MagicNumber = 0xbef5c

// Option 对于 GeeRPC 来说，目前需要协商的唯一一项内容是消息的编解码方式
type Option struct {
	MagicNumber int        // MagicNumber marks this a geerpc request
	CodecType   codec.Type // 编码方式\

	// 超时处理
	ConnectTimeout time.Duration
	HandleTimeout  time.Duration
}

// DefaultOption 为了实现上简单，Option采用JSON编码，后续的Header和body采用选择的编码方式
var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      codec.GobType,
	ConnectTimeout: time.Second * 10, // 10s 的超时时间
}

// Server Represent an RPC server
type Server struct {
	serviceMap sync.Map
}

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
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
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
		wg.Add(1)                                         // 接收并处理请求
		go server.handleRequest(cc, req, sending, wg, 10) // 处理请求
	}
	wg.Wait()
	_ = cc.Close()
}

// request stores all information of a call
type request struct {
	h            *codec.Header // header of request
	argv, replyv reflect.Value // argv and replyv of request
	mtype        *methodType
	svc          *service
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
	// ------------------------todo--------------------------
	//req.argv = reflect.New(reflect.TypeOf(""))
	req.svc, req.mtype, err = server.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	// 构造两个入参实例
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()

	// make sure that argvi is a pointer, ReadBody need a pointer as parameter
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface() // 先转换成指针
	}
	// ----------------------------------------------
	// 读取连接中的参数信息（传输主体就是参数列表）, 并存放到req.argv.Interface()中
	// 通过 cc.ReadBody() 将请求报文反序列化为第一个入参 argv
	if err = cc.ReadBody(argvi); err != nil {
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

func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	//// day 1, just print argv and send a hello message
	//defer wg.Done()
	//
	////log.Println(req.h, req.argv.Elem())
	////req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.h.Seq))
	//// ----fTODO: should call registered rpc methods to get the right replyv------
	//err := req.svc.call(req.mtype, req.argv, req.replyv) // 方法调用
	//if err != nil {
	//	req.h.Error = err.Error()
	//	server.sendResponse(cc, req.h, invalidRequest, sending)
	//	return
	//}
	//// ----------------------------------------------------
	//server.sendResponse(cc, req.h, req.replyv.Interface(), sending)

	// **添加服务器超时处理**

	defer wg.Done()
	// 需要确保 sendResponse 仅调用一次
	// 这里的管道用作为信号机制（提示某一步完成，实际上没有数据被发送）
	// 使用struct是因为struct{}是内存占用最小的数据类型
	//since it contains literally nothing, so no allocation necessary
	called := make(chan struct{})
	sent := make(chan struct{})
	go func() {
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		called <- struct{}{}
		if err != nil {
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, invalidRequest, sending)
			sent <- struct{}{}
			return
		}
		server.sendResponse(cc, req.h, req.replyv.Interface(), sending)
		sent <- struct{}{}
	}()

	if timeout == 0 {
		<-called // 等待子协程完成
		<-sent
		return
	}
	select {
	//time.After() 先于 called 接收到消息，说明处理已经超时，called 和 sent 都将被阻塞。
	//在 case <-time.After(timeout) 处调用 sendResponse。
	case <-time.After(timeout): // 超时
		req.h.Error = fmt.Sprintf("rpc server: request handle timeout: expect within %s", timeout)
		server.sendResponse(cc, req.h, invalidRequest, sending)
	case <-called: // called 信道接收到消息，代表处理（call函数）没有超时，继续执行 sendResponse。
		<-sent
	}
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

/**
通过反射结构体已经映射为服务，但请求的处理过程还没有完成。从接收到请求到回复还差以下几个步骤：
第一步，根据入参类型，将请求的 body 反序列化；
第二步，调用 service.call，完成方法调用；
第三步，将 reply 序列化为字节流，构造响应报文，返回。
*/

// Register publishes in the server the set of methods of the
// receiver value that satisfy the following conditions:
//   - exported method of exported type
//   - two arguments, both of exported type
//   - the second argument is a pointer
//   - one return value, of type error
//
// 将方法注册到server中的方法map中，如果存在则报错
func (server *Server) Register(rcvr interface{}) error {
	s := newService(rcvr)
	// 多线程的情况下，同时操作map也是有风险的（因为会有插入操作）
	// LoadOrStore: 如果键存在，则执行load，否则执行store. dup: 是否执行了load（此时键存在）
	if _, dup := server.serviceMap.LoadOrStore(s.name, s); dup {
		return errors.New("rpc: service already defined: " + s.name)
	}
	return nil
}

// Register publishes the receiver's methods in the DefaultServer.
func Register(rcvr interface{}) error { return DefaultServer.Register(rcvr) }

// 通过 ServiceMethod 从 serviceMap 中找到对应的 service
// ServiceMethod 的构成是 “Service.Method”
func (server *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".") // 子串在字符串中最后一次出现的位置
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svci, ok := server.serviceMap.Load(serviceName) // 传入键返回map中的值，返回service类型结构体
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	svc = svci.(*service) // 类型转换， type assertion
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find method " + methodName)
	}
	return
}

/*
*
day5 HTTP support
*/

const (
	connected        = "200 Connected to Gee RPC"
	defaultRPCPath   = "/_geeprc_"
	defaultDebugPath = "/debug/geerpc" // 为后续 DEBUG 页面预留的地址。
)

// @Description: ServeHTTP implements a http.Handler that answers RPC requests.(转换HTTP报头适配RPC报头)
// @receiver server
// @param w： 用于构造针对该请求的响应
// @param req：该对象包含该HTTP请求的所有信息
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed) // 设置状态码
		_, _ = io.WriteString(w, "405 must CONNECT\n")
		return
	}
	// 这是一段接管 HTTP 连接的代码，所谓的接管 HTTP 连接是指这里接管了 HTTP 的 Socket 连接，
	//也就是说 Golang 的内置 HTTP 库和 HTTPServer 库将不会管理这个 Socket 连接的生命周期，
	//这个生命周期已经划给 Hijacker 了（可用于实现一个不同的应用协议，指定HTTP的一些功能，如keep-alive）
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	_, _ = io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")
	server.ServeConn(conn)
}

// HandleHTTP registers an HTTP handler for RPC messages on rpcPath,
// and a debugging handler on debugPath.
// It is still necessary to invoke http.Serve(), typically in a go statement.
// 对于不同目录下的相应
func (server *Server) HandleHTTP() {
	http.Handle(defaultRPCPath, server)
	http.Handle(defaultDebugPath, debugHTTP{server})
	log.Println("rpc server debug path:", defaultDebugPath)
}

// HandleHTTP is a convenient approach for default server to register HTTP handlers
func HandleHTTP() {
	DefaultServer.HandleHTTP()
}
