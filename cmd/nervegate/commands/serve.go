package commands

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hxmdxnx/nervegate/pkg/classifier"
	"github.com/hxmdxnx/nervegate/pkg/ingress"
	"github.com/hxmdxnx/nervegate/pkg/rotator"
	"github.com/hxmdxnx/nervegate/pkg/trimmer"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var port int
	var socketPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the NerveGate proxy daemon",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("====================================================\n")
			fmt.Printf("  NerveGate Daemon\n")
			fmt.Printf("  The Sub-Millisecond Gateway & Model Orchestrator\n")
			fmt.Printf("====================================================\n")
			fmt.Printf("[+] HTTP Port:   :%d\n", port)
			fmt.Printf("[+] Unix Socket: %s\n", socketPath)

			cls := classifier.New()
			rot := rotator.New()
			trm := trimmer.New()

			srv := ingress.NewServer(ingress.Config{
				Port:       port,
				SocketPath: socketPath,
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
				log.Fatalf("[-] Server error: %v", err)
			}
			fmt.Println("[+] NerveGate daemon stopped cleanly.")
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "HTTP server listening port")
	cmd.Flags().StringVarP(&socketPath, "socket", "s", "/tmp/nervegate.sock", "Unix domain socket path")
	return cmd
}
