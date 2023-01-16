package codec

import "io"

// Header type: 定义类型别名
type Header struct {
	ServiceMethod string // 服务名&方法名
	Seq           uint64 // 请求序列
	Error         string // 错误信息
}

// Codec 抽象出对消息体进行编解码的接口 Codec，抽象出接口是为了实现不同的 Codec 实例
type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

type NewCodecFunc func(io.ReadWriteCloser) Codec // Codec 构造函数

type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json"
)

var NewCodecFuncMap map[Type]NewCodecFunc // 通过type获取对应的构造函数

func init() { // 初始化map
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}
