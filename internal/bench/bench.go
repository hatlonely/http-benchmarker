package bench

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// NewBenchmarkerBuilder new benchmarker builder
func NewBenchmarkerBuilder() *BenchmarkerBuilder {
	return &BenchmarkerBuilder{
		TimeDistributionThreshold: []time.Duration{
			time.Duration(50) * time.Millisecond,
			time.Duration(100) * time.Millisecond,
			time.Duration(200) * time.Millisecond,
			time.Duration(300) * time.Millisecond,
			time.Duration(500) * time.Millisecond,
		},
	}
}

// BenchmarkerBuilder builder
type BenchmarkerBuilder struct {
	WorkerNum                 int
	TimeDistributionThreshold []time.Duration
	Filename                  string
}

// WithWorkerNum option
func (b *BenchmarkerBuilder) WithWorkerNum(workerNum int) *BenchmarkerBuilder {
	b.WorkerNum = workerNum
	return b
}

// WithTimeDistributionThreshold option
func (b *BenchmarkerBuilder) WithTimeDistributionThreshold(timeDistributionThreshold []time.Duration) *BenchmarkerBuilder {
	b.TimeDistributionThreshold = timeDistributionThreshold
	return b
}

// WithFilename option
func (b *BenchmarkerBuilder) WithFilename(filename string) *BenchmarkerBuilder {
	b.Filename = filename
	return b
}

// Build Benchmarker
func (b *BenchmarkerBuilder) Build() (*Benchmarker, error) {
	fp, err := os.Open(b.Filename)
	defer fp.Close()
	reader := bufio.NewReader(fp)
	if err != nil {
		logrus.WithFields(logrus.Fields{"error": err, "type": "bench"}).Error()
		return nil, err
	}
	var infos []*URLInfo
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				logrus.WithFields(logrus.Fields{"error": err, "type": "bench"}).Warn()
			}
			break
		}
		infos = append(infos, NewURLInfo(line[:len(line)-1]))
	}

	return &Benchmarker{
		workerNum:                 b.WorkerNum,
		timeDistributionThreshold: b.TimeDistributionThreshold,
		infos: infos,
	}, nil
}

// NewURLInfo info
func NewURLInfo(line string) *URLInfo {
	return &URLInfo{
		url: line,
	}
}

// URLInfo url info
type URLInfo struct {
	method string
	url    string
	header map[string]string
	data   string
}

// Benchmarker benchmarker
type Benchmarker struct {
	workerNum                 int
	timeDistributionThreshold []time.Duration
	infos                     []*URLInfo
}

// Benchmark run benchmark
func (b *Benchmarker) Benchmark() error {
	timeDisArrayStr := make([]string, len(b.timeDistributionThreshold))
	for i := range b.timeDistributionThreshold {
		timeDisArrayStr[i] = fmt.Sprintf("%v", b.timeDistributionThreshold[i])
	}
	fmt.Printf("\t%v\t%v\t%v\t% 8v\t% 8v\t% 8v\t%v\n", "succ", "fail", "totalTime", "qps", "res_time", strings.Join(timeDisArrayStr, "\t"), `succ%`)
	l := len(b.infos)
	var wg sync.WaitGroup
	kpis := make(chan *KPI, b.workerNum)
	for i := 0; i < b.workerNum; i++ {
		go func(i int) {
			kpis <- b.BenchmarkOnce(b.infos[i*l/b.workerNum : (i+1)*l/b.workerNum])
			wg.Done()
		}(i)
		wg.Add(1)
	}
	wg.Wait()
	close(kpis)

	kpiMap := map[string]*KPI{}
	for kpi := range kpis {
		if _, ok := kpiMap[kpi.name]; !ok {
			kpiMap[kpi.name] = &KPI{kpi.name, 0, 0, 0, 0, make([]int, len(b.timeDistributionThreshold))}
		}
		kpiMap[kpi.name].success += kpi.success
		kpiMap[kpi.name].fail += kpi.fail
		kpiMap[kpi.name].totalTime += kpi.totalTime
		kpiMap[kpi.name].count += kpi.count
		for i := range kpi.timeDistribution {
			kpiMap[kpi.name].timeDistribution[i] += kpi.timeDistribution[i]
		}
	}

	for _, kpi := range kpiMap {
		kpi.name = fmt.Sprintf("%v-%v", kpi.name, b.workerNum)
		fmt.Println(kpi.Show())
	}

	return nil
}

// BenchmarkOnce benchmark once
func (b *Benchmarker) BenchmarkOnce(infos []*URLInfo) *KPI {
	totalTime := time.Duration(0)
	success := 0
	fail := 0
	timeDistribution := make([]int, len(b.timeDistributionThreshold))
	for _, info := range infos {
		ts := time.Now()
		resp, err := http.Get(info.url)
		// resp, err := http.Get("http://www.baidu.com")
		_ = info
		if err != nil {
			fail++
			continue
		}
		resp.Body.Close()
		elaspe := time.Since(ts)
		totalTime += elaspe
		success++
		for i := range timeDistribution {
			if elaspe < b.timeDistributionThreshold[i] {
				timeDistribution[i]++
			}
		}
	}

	return &KPI{"http", success, fail, totalTime, 1, timeDistribution}
}

// KPI key point index
type KPI struct {
	name             string
	success          int
	fail             int
	totalTime        time.Duration
	count            int
	timeDistribution []int
}

// Show KPI on console
func (k *KPI) Show() string {
	timeDistributionPercent := make([]string, len(k.timeDistribution))
	for i := range k.timeDistribution {
		timeDistributionPercent[i] = fmt.Sprintf("%.5f", float64(k.timeDistribution[i])/float64(k.fail+k.success))
	}

	return fmt.Sprintf(
		"%v\t%v\t%v\t% 8v\t% 8v\t% 8v\t%v\t%v",
		k.name, k.success, k.fail, k.totalTime,
		k.success*int(time.Second)*k.count/int(k.totalTime),
		k.totalTime/time.Duration(k.success),
		strings.Join(timeDistributionPercent, "\t"),
		float64(k.success)/float64(k.fail+k.success),
	)
}
