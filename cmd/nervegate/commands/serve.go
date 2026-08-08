package commands

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/dhanizael/nervegate/pkg/classifier"
	"github.com/dhanizael/nervegate/pkg/ingress"
	"github.com/dhanizael/nervegate/pkg/rotator"
	"github.com/dhanizael/nervegate/pkg/trimmer"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var port int
	var socketPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the NerveGate proxy daemon",
		Run: func(cmd *cobra.Command, args []string) {
			provider := envOr("NERVEGATE_PROVIDER", "openai")

			fmt.Printf("====================================================\n")
			fmt.Printf("  NerveGate Daemon\n")
			fmt.Printf("  The Sub-Millisecond Gateway & Model Orchestrator\n")
			fmt.Printf("====================================================\n")
			fmt.Printf("[+] HTTP Port:   :%d\n", port)
			fmt.Printf("[+] Unix Socket: %s\n", socketPath)
			fmt.Printf("[+] Provider:    %s\n", provider)

			cls := classifier.New()
			rot := rotator.New()
			trm := trimmer.New()

			// Load the provider key pool from the environment:
			//   NERVEGATE_KEYS_<PROVIDER>  comma-separated API keys
			//   NERVEGATE_RPM_<PROVIDER>   per-key requests/minute limit (0 = unlimited)
			//   NERVEGATE_TPM_<PROVIDER>   per-key tokens/minute limit (0 = unlimited)
			// When no pool is configured, requests pass through the caller's
			// own Authorization header instead.
			loadKeyPool(rot, provider)

			srv := ingress.NewServer(ingress.Config{
				Port:       port,
				SocketPath: socketPath,
				Provider:   provider,
				Upstream:   envOr("NERVEGATE_UPSTREAM", ""),
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

// loadKeyPool reads NERVEGATE_KEYS_<PROVIDER> and registers each key with the
// rotator, applying optional RPM/TPM limits.
func loadKeyPool(rot *rotator.KeyRotator, provider string) {
	upper := strings.ToUpper(provider)
	raw := strings.TrimSpace(os.Getenv("NERVEGATE_KEYS_" + upper))
	if raw == "" {
		fmt.Println("[+] No key pool configured; forwarding caller Authorization headers.")
		return
	}

	rpm := 0
	if v := strings.TrimSpace(os.Getenv("NERVEGATE_RPM_" + upper)); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			fmt.Printf("[!] Invalid NERVEGATE_RPM_%s value %q; defaulting to 0 (unlimited)\n", upper, v)
		} else {
			rpm = parsed
		}
	}
	tpm := 0
	if v := strings.TrimSpace(os.Getenv("NERVEGATE_TPM_" + upper)); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			fmt.Printf("[!] Invalid NERVEGATE_TPM_%s value %q; defaulting to 0 (unlimited)\n", upper, v)
		} else {
			tpm = parsed
		}
	}

	count := 0
	for _, k := range strings.Split(raw, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		count++
		rot.AddKey(provider, &rotator.APIKey{
			ID:       fmt.Sprintf("%s-%d", provider, count),
			Key:      k,
			Provider: provider,
			RPM:      rpm,
			TPM:      tpm,
		})
	}
	fmt.Printf("[+] Key pool for '%s': %d key(s), RPM=%d TPM=%d\n", provider, count, rpm, tpm)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
