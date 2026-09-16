package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"wind-agent/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, "./conf/config.yaml"); err != nil {
		panic(err)
	}
	// 优雅退出
	//select {
	//case <-ctx.Done():
	//	fmt.Println("interrupted")
	//}
}
