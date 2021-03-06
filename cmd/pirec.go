package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/trezz/pirec/pkg/pirec"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("usage: %v FILE\n", os.Args[0])
		return
	}

	mon, err := pirec.CreateMonitor(os.Args[1])
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	monCtx, cancelMon := context.WithCancel(ctx)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-sigc
		fmt.Println("pirec: Signal received: ", s)
		cancelMon()
	}()

	err = mon.Start(monCtx)
	if err != nil {
		panic(err)
	}
}
