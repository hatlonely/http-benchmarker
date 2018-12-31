package main

import (
	"fmt"
	"os"

	"github.com/hatlonely/http-benchmarker/internal/bench"
	"github.com/spf13/pflag"
)

// AppVersion version
var AppVersion = "unknown"

func main() {
	version := pflag.BoolP("version", "v", false, "print current version")
	workerNum := pflag.IntP("workerNum", "n", 3, "worker numebr")
	filename := pflag.StringP("filename", "f", "", "filename")
	pflag.Parse()
	if *version {
		fmt.Println(AppVersion)
		os.Exit(0)
	}

	benchmarker, err := bench.NewBenchmarkerBuilder().
		WithWorkerNum(*workerNum).
		WithFilename(*filename).
		Build()
	if err != nil {
		panic(err)
	}

	if err := benchmarker.Benchmark(); err != nil {
		panic(err)
	}
}
