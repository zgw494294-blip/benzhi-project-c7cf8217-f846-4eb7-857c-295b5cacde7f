package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/httpui"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/selfcheck"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/storage"
)

func main() {
	addressFlag := flag.String("addr", "", "监听地址，例如 127.0.0.1:19081")
	dataSource := flag.String("data", "collation.db", "SQLite 数据源路径，内存模式使用 :memory:")
	selfCheck := flag.Bool("selfcheck", false, "运行有界端到端自检后退出")
	flag.Parse()
	address, err := resolveAddress(*addressFlag)
	if err != nil {
		log.Fatalf("配置错误: %v", err)
	}
	store, err := storage.Open(*dataSource)
	if err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}
	defer store.Close()
	app := application.NewService(store)
	if *selfCheck {
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		result, err := selfcheck.Run(ctx, address, app)
		if err != nil {
			log.Fatalf("自检失败: %v", err)
		}
		fmt.Printf("自检通过：卷 %s，入藏清单 %s，审计事件 %d 条，冻结摘要 %s\n", result.VolumeID, result.ManifestID, result.AuditEventCount, result.FrozenDigest)
		return
	}
	server := &http.Server{Addr: address, Handler: httpui.New(app).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 90 * time.Second}
	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownSignal.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	log.Printf("古籍数字校勘入藏台已启动：http://%s", address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP 服务异常退出: %v", err)
	}
}
