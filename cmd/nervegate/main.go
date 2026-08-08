package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hxmdxnx/nervegate/pkg/classifier"
	"github.com/hxmdxnx/nervegate/pkg/ingress"
	"github.com/hxmdxnx/nervegate/pkg/rotator"
	"github.com/hxmdxnx/nervegate/pkg/trimmer"
)

const (
	Version = "0.1.0-alpha"
	Tagline = "The Sub-Millisecond Intelligent Gateway & Model Orchestrator for Linux"
)

func main() {
	portFlag := flag.Int("port", 8080, "HTTP server port")
	socketFlag := flag.String("socket", "/tmp/nervegate.sock", "Unix Domain Socket path")
	verFlag := flag.Bool("version", false, "Print version and exit")

	flag.Parse()

	if *verFlag {
		fmt.Printf("NerveGate v%s - %s\n", Version, Tagline)
		os.Exit(0)
	}

	fmt.Printf("====================================================\n")
	fmt.Printf("  NerveGate v%s\n", Version)
	fmt.Printf("  %s\n", Tagline)
	fmt.Printf("====================================================\n")
	fmt.Printf("[+] Initializing Microsecond Ingress Engine...\n")
	fmt.Printf("[+] HTTP Listener: :%d\n", *portFlag)
	fmt.Printf("[+] Unix Socket:   %s\n", *socketFlag)

	cls := classifier.New()
	rot := rotator.New()
	trm := trimmer.New()

	srv := ingress.NewServer(ingress.Config{
		Port:       *portFlag,
		SocketPath: *socketFlag,
	}, cls, rot, trm)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("\n[-] Received signal %v. Shutting down NerveGate gracefully...\n", sig)
		cancel()
	}()

	if err := srv.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("[-] Server runtime error: %v", err)
	}

	fmt.Println("[+] NerveGate shutdown complete.")
}
