package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	address          string
	dataDirectory    string
	selfcheck        bool
	selfcheckTimeout time.Duration
}

func parseConfig(arguments []string) (config, error) {
	defaultAddress, err := addressFromEnvironment()
	if err != nil {
		return config{}, err
	}
	set := flag.NewFlagSet("trapreview", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	value := config{}
	set.StringVar(&value.address, "addr", defaultAddress, "HTTP 监听地址")
	set.StringVar(&value.dataDirectory, "data-dir", "./data", "持久化数据目录")
	set.BoolVar(&value.selfcheck, "selfcheck", false, "运行完整业务自检后退出")
	set.DurationVar(&value.selfcheckTimeout, "selfcheck-timeout", 15*time.Second, "自检超时")
	if err := set.Parse(arguments); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("不支持的位置参数: %s", strings.Join(set.Args(), " "))
	}
	if err := validateAddress(value.address); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(value.dataDirectory) == "" {
		return config{}, errors.New("data-dir 不能为空")
	}
	if value.selfcheckTimeout <= 0 {
		return config{}, errors.New("selfcheck-timeout 必须大于零")
	}
	return value, nil
}

func addressFromEnvironment() (string, error) {
	portValue := strings.TrimSpace(os.Getenv("PORT"))
	if portValue == "" {
		return "127.0.0.1:19081", nil
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("PORT 必须是 1 到 65535 之间的端口号")
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("addr 必须是 host:port: %w", err)
	}
	if host == "" {
		return errors.New("addr 必须明确指定监听主机")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("addr 端口必须在 1 到 65535 之间")
	}
	return nil
}
