package main

import (
	"flag"
	"github.com/go-yaml/yaml"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io/ioutil"
	rehook "rehook/pkg"
	"time"
)

type Config struct {
	Rules map[string]string
}

func main() {
	addr := flag.String("addr", "", "Server websocket addr, example: wss://example.com:8080/")
	rulepath := flag.String("rules", "", "Path to file with retransmit rules")
	reconnTimeout := flag.Int("timeout", 5, "Reconnect timeout is seconds")
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

	if *rulepath == "" {
		log.Panic("missing rules file")
	}

	cfgdata, err := ioutil.ReadFile(*rulepath)
	if err != nil {
		log.Panic("failed read config file", zap.Error(err))
	}

	var cfg Config
	if err := yaml.Unmarshal(cfgdata, &cfg); err != nil {
		log.Panic("failed parse config file", zap.Error(err))
	}

	client := rehook.NewClient(log, time.Duration(*reconnTimeout)*time.Second, cfg.Rules)
	if err := client.Tunnel(*addr); err != nil {
		log.Panic("failed tunnel from client to server", zap.Error(err))
	}
}
