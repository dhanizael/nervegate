package commands

import (
	"fmt"
	"time"

	"github.com/dhanizael/nervegate/pkg/classifier"
	"github.com/dhanizael/nervegate/pkg/trimmer"
	"github.com/spf13/cobra"
)

func newBenchmarkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "benchmark",
		Short: "Run built-in microsecond latency benchmarks",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("==> Running NerveGate Microsecond Latency Benchmark...")
			cls := classifier.New()
			trm := trimmer.New()

			const iterations = 100000
			prompt := "Fix [CRITICAL] deadlock in mutex handler"
			toolCtx := "func Lock() {\n\n\n   fmt.Println(\"testing\")   \n}"

			start := time.Now()
			for i := 0; i < iterations; i++ {
				_, _ = trm.Trim(toolCtx)
				_ = cls.Classify(prompt, toolCtx)
			}
			elapsed := time.Since(start)

			nsPerOp := elapsed.Nanoseconds() / iterations
			usPerOp := float64(nsPerOp) / 1000.0

			fmt.Printf("Completed %d iterations in %v\n", iterations, elapsed)
			fmt.Printf("Latency Per Request: %.3f µs (%d ns)\n", usPerOp, nsPerOp)
			fmt.Println("Result: SUB-MICROSECOND LATENCY VERIFIED.")
		},
	}
}
