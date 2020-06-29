package main

import (
	"go.uber.org/zap"
	"net/http"
)

func main() {
	log, _ := zap.NewDevelopment()
	http.HandleFunc("/test", func(writer http.ResponseWriter, request *http.Request) {
		log.Info("Incoming message", zap.Any("request", request))
		writer.Write([]byte("hello world"))
	})
	http.ListenAndServe(":8182", nil)
}
