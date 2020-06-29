package main

import (
	"flag"
	"fmt"
	rehook "github.com/sintanial/go-rehook/pkg"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := flag.String("addr", "", "Server listen addr, example: :8080")
	certFile := flag.String("cert", "", "Path to public chain certificate")
	keyFile := flag.String("key", "", "Path to private certificate")
	retransmitTimeout := flag.Int("timeout", 0, "Retransmit timeout from server to client in seconds, zero means infinity timeout")
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
		fmt.Println("missing addr")
		os.Exit(1)
	}

	server := rehook.NewServer(log, time.Duration(*retransmitTimeout)*time.Second)

	log.Info("listen and serve", zap.String("addr", *addr))

	handler := http.NewServeMux()
	if *certFile != "" {
		err = server.ListenAndServeTLS(*addr, *certFile, *keyFile, handler)
	} else {
		err = server.ListenAndServe(*addr, handler)
	}

	log.Fatal("failed serve", zap.Error(err))
}
