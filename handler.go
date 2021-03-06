package jwa

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

var ServeMux *http.ServeMux
var urlPrefix string

type ProcedureCallbackFunc func(data map[interface{}]interface{}, writer http.ResponseWriter, request *http.Request) bool
type ProcedureFunc func(data map[interface{}]interface{}, payload []byte) interface{}

// 下面两个全局回调函数会注册到handler内的同名成员中
var preCallbacks []ProcedureCallbackFunc
var postCallbacks []ProcedureCallbackFunc

// 添加消息处理后就不能再调用函数设置 preCallbacks/postCallbacks
// 因为添加消息时候会拷贝这两个变量 如果添加消息之后再设置 通常会造成难以发现的BUG
// 所以这个变量用来检查判断 如果遇到则直接panic 编译期暴露错误
var messageHandlerAdded bool

// 由于框架要求使用者使用func init()的方式自动注册消息
// 所以框架的初始化是在消息注册函数之后
// 因此需要这里先保存下所有注册的handler
// 然后再启动的时候再执行实际注册操作
var registeredHandler []handler

type handler struct {

	// 注册消息的名字，用来注册HTTP-URL /name 的POST
	name string

	// 消息的一些选项
	options HandlerOptions

	// 消息的处理回调函数
	procedure ProcedureFunc

	/* 下面两个回调函数在proc调用前后执行 ** data的生命周期是每个proc ** 可用来在回调函数前后传递记录数据 */

	// 消息处理前的回调函数 返回bool值代表是否继续往下执行后续procedure过程
	preCallbacks []ProcedureCallbackFunc

	// 消息处理后的回调函数 返回bool值代表是否继续往下执行后续postCallbacks
	postCallbacks []ProcedureCallbackFunc
}

type HandlerOptions struct {
	ClearPreCallbacks  bool
	ClearPostCallbacks bool

	preCallbacks  []ProcedureCallbackFunc
	postCallbacks []ProcedureCallbackFunc
}

func init() {
	ServeMux = http.NewServeMux()
	urlPrefix = "/"
}

func (h handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {

	data := make(map[interface{}]interface{})

	internalPreProcedure(data, request)

	for _, cb := range h.preCallbacks {
		if !cb(data, writer, request) {
			return
		}
	}

	var payload []byte
	var err error

	payload, err = ioutil.ReadAll(request.Body)

	if err != nil {
		writer.WriteHeader(http.StatusExpectationFailed)
		return
	}

	reply := h.procedure(data, payload)

	var response []byte
	err = nil

	switch v := reply.(type) {
	case int:
		writer.WriteHeader(v)
		return

	default:
		response, err = json.Marshal(v)
		if err != nil {
			// 通常是编程错误 应该在调试期间就解决掉
			panic(err)
		}

		_, err := writer.Write(response)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	for _, cb := range h.postCallbacks {
		if !cb(data, writer, request) {
			break
		}
	}

	internalPostProcedure(data, request)

	return
}

// TODO 基于下面两个函数进行消息处理时间统计等操作

func internalPreProcedure(data map[interface{}]interface{}, request *http.Request) {

}

func internalPostProcedure(data map[interface{}]interface{}, request *http.Request) {

}

func (options *HandlerOptions) AddPreProcedureCallback(cb ProcedureCallbackFunc) {

	options.preCallbacks = append(options.preCallbacks, cb)
}

func (options *HandlerOptions) AddPostProcedureCallback(cb ProcedureCallbackFunc) {

	options.postCallbacks = append(options.postCallbacks, cb)
}

func SetURLPrefix(prefix string) {

	if len(prefix) == 0 {
		panic("SetURLPrefix Got Bad Prefix")
	}

	if prefix[:1] != "/" {
		panic("SetURLPrefix must be starts with / (got " + prefix + ")")
	}

	if strings.HasSuffix(prefix, "/") {
		urlPrefix = prefix
	} else {
		urlPrefix = prefix + "/"
	}

}

func AddPreProcedureCallback(cb ProcedureCallbackFunc) {

	if messageHandlerAdded {
		panic("AddPreProcedureCallback must be called before MessageHandler Added")
	}

	if serverRunning {
		panic("AddPreProcedureCallback must be called before StartServer")
	}

	preCallbacks = append(preCallbacks, cb)
}

func AddPostProcedureCallback(cb ProcedureCallbackFunc) {

	if messageHandlerAdded {
		panic("AddPostProcedureCallback must be called before MessageHandler Added")
	}

	if serverRunning {
		panic("AddPostProcedureCallback must be called before StartServer")
	}

	postCallbacks = append(postCallbacks, cb)
}

func AddMessageHandler(name string, procedure ProcedureFunc) {

	options := HandlerOptions{}

	AddMessageHandlerExt(name, procedure, options)
}

func AddMessageHandlerExt(name string, procedure ProcedureFunc, options HandlerOptions) {

	if ServeMux == nil {
		panic("ServeMux == nil")
	}

	if serverRunning {
		panic("AddMessageHandler(Ext) must be called before StartServer")
	}

	registeredHandler = append(registeredHandler, handler{
		name:      name,
		procedure: procedure,
		options:   options,
	})
}

func registerHandler() {

	for _, h := range registeredHandler {

		handler := handler{
			name:      h.name,
			options:   h.options,
			procedure: h.procedure,
		}

		if !handler.options.ClearPreCallbacks {
			handler.preCallbacks = preCallbacks
		}

		handler.preCallbacks = append(handler.preCallbacks, handler.options.preCallbacks...)

		if !handler.options.ClearPostCallbacks {
			handler.postCallbacks = postCallbacks
		}

		handler.postCallbacks = append(handler.postCallbacks, handler.options.postCallbacks...)

		url := fmt.Sprintf("%s%s", urlPrefix, handler.name)

		// panic if url dup
		ServeMux.Handle(url, handler)

		fmt.Printf("JWA %s\n", url)

	}

	messageHandlerAdded = true
}
