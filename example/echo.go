package main

import (
	"encoding/json"
	"jwa"
)

type echoRequest struct {
	Token string
}

type echoReply struct {
	Result      int
	Description string

	// Add struct fields with capital letters ...
}

func init() {
	jwa.AddMessageHandler("echo", echoHandler)
}

func echoHandler(data map[interface{}]interface{}, payload []byte) (resp interface{}) {

	var msg echoRequest

	if err := json.Unmarshal(payload, &msg); err != nil {
		return jwa.StatusBadRequest
	}

	reply := &echoReply{
		Result:      0,
		Description: "OK",
	}

	// write code following here now !

	return reply
}
