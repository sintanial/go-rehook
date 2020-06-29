package main

import (
	"flag"
	"fmt"
	"github.com/go-yaml/yaml"
	rehook "github.com/sintanial/go-rehook/pkg"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io/ioutil"
	"os"
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
		exit("missing addr")
	}

	if *rulepath == "" {
		exit("missing rules file")
	}

	cfgdata, err := ioutil.ReadFile(*rulepath)
	if err != nil {
		exit("failed read config file: " + err.Error())
	}

	var cfg Config
	if err := yaml.Unmarshal(cfgdata, &cfg); err != nil {
		exit("failed parse config file: " + err.Error())
	}

	client := rehook.NewClient(log, time.Duration(*reconnTimeout)*time.Second, cfg.Rules)
	if err := client.Tunnel(*addr); err != nil {
		exit("failed tunnel from client to server: " + err.Error())
	}
}

func exit(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}