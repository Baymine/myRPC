// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/**
在 /debug/geerpc 上展示服务的调用统计视图。
*/

package geerpc

import (
	"fmt"
	"html/template"
	"net/http"
)

// 这个报文将展示注册所有的 service 的每一个方法的调用情况。
const debugText = `<html>
	<body>
	<title>GeeRPC Services</title>
	{{range .}}
	<hr>
	Service {{.Name}}
	<hr>
		<table>
		<th align=center>Method</th><th align=center>Calls</th>
		{{range $name, $mtype := .Method}}
			<tr>
			<td align=left font=fixed>{{$name}}({{$mtype.ArgType}}, {{$mtype.ReplyType}}) error</td>
			<td align=center>{{$mtype.NumCalls}}</td>
			</tr>
		{{end}}
		</table>
	{{end}}
	</body>
	</html>`

// html/template provides the same API but has additional security features and
// should be used for generating HTML.
// 'template.Must' function to panic in case Parse returns an error.

// Templates are a mix of static text and “actions” enclosed in {{...}}
//that are used to dynamically insert content. 可以看到debugText中的{{。。。}}

// template.new(): allocates a new HTML template with the given name.

// 实际上就是一个动态文本，占位符({{..}})可以后续填上
var debug = template.Must(template.New("RPC debug").Parse(debugText))

type debugHTTP struct {
	*Server
}

type debugService struct {
	Name   string
	Method map[string]*methodType
}

// Runs at /debug/geerpc
func (server debugHTTP) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Build a sorted version of the data.
	var services []debugService
	// 如果func返回false则停止遍历（条件遍历map），这里将map中的键值对作为函数的两个参数传入
	server.serviceMap.Range(func(namei, svci interface{}) bool {
		svc := svci.(*service) // 类型转换
		services = append(services, debugService{
			Name:   namei.(string),
			Method: svc.method,
		})
		return true
	})
	err := debug.Execute(w, services) // 将service解析然后写入w中
	if err != nil {
		_, _ = fmt.Fprintln(w, "rpc: error executing template:", err.Error())
	}
}
