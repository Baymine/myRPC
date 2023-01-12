package geerpc

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

// 包含了一个方法的完整信息
type methodType struct {
	method    reflect.Method
	ArgType   reflect.Type // 第一个参数类型
	ReplyType reflect.Type // 第二个参数类型
	numCalls  uint64       // 用于统计方法调用的次数
}

// NumCalls 返回成员变量numCalls
func (m *methodType) NumCalls() uint64 {
	// 64位数字读取的时候先读低32位，后读高32位，为了保证一致性，这里的原子操作有必要
	return atomic.LoadUint64(&m.numCalls)
}

// 生成methodType中的argv
func (m *methodType) newArgv() reflect.Value {
	var argv reflect.Value
	// arg may be a pointer type, or a value type
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem()) // 指针指向的位置的类型（类似数组元素的类型）
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

// 生成methodType中的replyv
func (m *methodType) newReplyv() reflect.Value {
	// reply must be a pointer type
	replyv := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem())) // 赋值，创建对应的map
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replyv
}

type service struct {
	name string       // 映射的结构体的名称
	typ  reflect.Type // 结构体的类型

	// rcvr：receiver
	rcvr   reflect.Value          // 结构体的实例本身，保留 rcvr 是因为在调用时需要 rcvr 作为第 0 个参数
	method map[string]*methodType // 存储映射的结构体的所有符合条件的方法。
}

// 构造函数,构造一个Service实例，传入的receiver用于获取类型
func newService(receiver interface{}) *service {
	s := new(service)
	s.rcvr = reflect.ValueOf(receiver)
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	s.typ = reflect.TypeOf(receiver)
	if !ast.IsExported(s.name) { // 是否大写开头（是否可调用）
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	s.registerMethods()
	return s
}

// 过滤出了符合条件的方法， 并依据提供的信息新建methodType
func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type

		// 两个导出或内置类型的入参（反射时为 3 个，第 0 个是自身，类似于 python 的 self，java 中的 this）
		// 返回值有且只有一个，类型为error
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		// 检查第0个是否是函数自身（this）
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}
		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, method.Name)
	}
}

// 能够通过反射值调用方法
func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.rcvr, argv, replyv}) // 调用f函数
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}
