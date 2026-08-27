package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trapreview/internal/application"
	"trapreview/internal/httpapi"
	"trapreview/internal/policy"
	"trapreview/internal/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(arguments []string) error {
	configuration, err := parseConfig(arguments)
	if err != nil {
		return err
	}
	dataDirectory := configuration.dataDirectory
	var cleanup func()
	if configuration.selfcheck {
		dataDirectory, err = os.MkdirTemp("", "trapreview-selfcheck-")
		if err != nil {
			return fmt.Errorf("创建自检数据目录: %w", err)
		}
		cleanup = func() { _ = os.RemoveAll(dataDirectory) }
		defer cleanup()
	}
	repository, err := store.Open(dataDirectory)
	if err != nil {
		return fmt.Errorf("打开持久化存储: %w", err)
	}
	service := application.NewService(repository, policy.NewQualityPolicy(), policy.NewReleasePolicy())
	server := httpapi.NewServer(configuration.address, httpapi.NewHandler(service))
	listener, err := net.Listen("tcp", configuration.address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", configuration.address, err)
	}
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()

	if configuration.selfcheck {
		ctx, cancel := context.WithTimeout(context.Background(), configuration.selfcheckTimeout)
		defer cancel()
		checkErr := runSelfcheck(ctx, "http://"+listener.Addr().String())
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		serveErr := <-serveResult
		if checkErr != nil {
			return checkErr
		}
		if shutdownErr != nil {
			return fmt.Errorf("关闭自检服务: %w", shutdownErr)
		}
		if serveErr != nil {
			return fmt.Errorf("自检服务异常: %w", serveErr)
		}
		log.Printf("野迹裁定台自检通过，发布凭据可成功核验")
		return nil
	}

	log.Printf("野迹裁定台监听 %s，数据目录 %s", listener.Addr(), dataDirectory)
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveResult:
		return err
	case <-signalContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownContext)
	}
}
