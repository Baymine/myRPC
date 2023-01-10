package codec

// Gob 是Go语言自己以二进制形式序列化和反序列化程序数据的格式，
// 可以在 encoding 包中找到。这种格式的数据简称为 Gob（即 Go binary 的缩写）

// gob包管理gob流——在编码器（发送器）和解码器（接受器）之间交换的binary值。
// 一般用于传递远端程序调用（RPC）的参数和结果，如net/rpc包就有提供。
import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	conn io.ReadWriteCloser // 由构建函数传入，通常是通过TCP或者Unix建立socket时得到的链接实例
	buf  *bufio.Writer      // 带缓冲是为了防止阻塞，能提升性能
	dec  *gob.Decoder       // gob 解码
	enc  *gob.Encoder       // gob 编码
}

// 确保接口被实现的常用方式
// 将nil强制转换成*GobCodec，再转换成Codec接口，如果转换失败
// 说明*GobCodec并没有实现Codec接口的所有方法
var _ Codec = (*GobCodec)(nil)

// CobType下的构造函数的实现
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	// 创建
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn), // 从网络连接conn中读取
		enc:  gob.NewEncoder(buf),  // 写入到缓冲buf中
	}
}

// GobCodec 实现接口Codec

// 解码报头
func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

// 解码主体
func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

// 编码报头和主体
func (c *GobCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		// bufio当缓存不足的时候，会将缓冲中的内容写入到文件中
		// 所以在程序结束的时候，一部分数据可能还会在缓冲中，这时候就需要手动将
		// 这些数据写入到文件中
		_ = c.buf.Flush() // 将缓冲中的所有数据
		if err != nil {
			_ = c.Close()
		}
	}()

	if err := c.enc.Encode(h); err != nil {
		log.Println("rpc codec: gob error encoding header:", err)
		return err
	}
	if err := c.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body:", err)
		return err
	}
	return nil
}

func (c *GobCodec) Close() error {
	return c.conn.Close()
}
