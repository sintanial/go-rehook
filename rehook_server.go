package main

import (
	"flag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
	rehook "rehook/pkg"
	"time"
)

func main() {
	addr := flag.String("addr", "", "Server listen addr, example: :8080")
	certFile := flag.String("cert", "", "Path to public chain certificate")
	keyFile := flag.String("key", "", "Path to private certificate")
	timeout := flag.Int("timeout", 0, "Retransmit timeout from server to client in seconds, zero means infinity timeout")
	flag.Parse()

	zapcfg := zap.NewDevelopmentConfig()
	zapcfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zapcfg.OutputPaths = []string{"stdout"}
	zapcfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	log, err := zapcfg.Build()
	if err != nil {
		panic(err)
	}

	if *addr == "" {
		log.Panic("missing addr")
	}

	server := rehook.NewServer(log, time.Duration(*timeout)*time.Second)

	log.Info("listen and serve", zap.String("addr", *addr))

	handler := http.NewServeMux()
	if *certFile != "" {
		err = server.ListenAndServeTLS(*addr, *certFile, *keyFile, handler)
	} else {
		err = server.ListenAndServe(*addr, handler)
	}

	log.Fatal("failed serve", zap.Error(err))
}
