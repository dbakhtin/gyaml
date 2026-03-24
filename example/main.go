package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/dbakhtin/gyaml"
)

var (
	original = flag.Bool("original", false, "use original go-yaml encoder")
	decode   = flag.Bool("decode", false, "benchmark decoder instead of encoder")
	prof     = flag.Bool("prof", false, "run profiler")
	v2       = flag.Bool("v2", false, "run version 2 benchmark")
)

// slice size
const size = 1000000

func init() {
	flag.Parse()
}

type TOC struct {
	StatisticsEntries []MetadataEntry
}

type MetadataEntry struct {
	Name  string
	Value string
	Other string
}

type TOCV2 struct {
	StatisticsEntries []MetadataEntryV2
}

type MetadataEntryV2 struct {
	Name    string
	Size    int64
	Volume  float64
	Enabled bool
	Since   time.Time
	Codes   []int `yaml:",flow"`
	Inf     float64
	Staff   map[string]string
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func generateSlice() TOC {
	toc := TOC{}
	fmt.Printf("Generating a slice of %d records\n", size)
	for i := range size {
		name := fmt.Sprintf("Name %v", i)
		value := fmt.Sprintf("Value %v", i)
		toc.StatisticsEntries = append(toc.StatisticsEntries, MetadataEntry{Name: name, Value: value})
	}
	return toc
}

func generateSliceV2() TOCV2 {
	toc := TOCV2{}
	fmt.Printf("Generating a slice of %d records\n", size)
	now := time.Now()
	for i := range size {
		me := MetadataEntryV2{
			Name:    fmt.Sprintf("Name %d", i),
			Size:    int64(i),
			Volume:  1.1 * float64(i),
			Enabled: i%2 == 0,
			Since:   now.Add(time.Hour),
			Codes:   []int{i / 3, i / 2, i, i + 5},
			Inf:     math.Inf(-1 + i%2),
			Staff:   map[string]string{"admin": fmt.Sprintf("Boris %d", i), "chief": fmt.Sprintf("BulletDodger %d", i)},
		}
		toc.StatisticsEntries = append(toc.StatisticsEntries, me)
	}
	return toc
}

func runEncoder(toc TOC) {
	file, _ := os.OpenFile(path.Join("example", "test.yaml"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	defer file.Close()
	// refresh GC
	runtime.GC()
	start := time.Now()
	if *original {
		enc := yaml.NewEncoder(file)
		fmt.Println("Encoding slice into example/test.yaml file")
		err := enc.Encode(toc)
		log.Printf("Time spent encoding: %s\n", time.Since(start))
		panicOnErr(err)
	} else {
		enc := gyaml.NewEncoder(file)
		fmt.Println("Encoding slice into example/test.yaml file")
		err := enc.Encode(toc)
		log.Printf("Time spent encoding: %s\n", time.Since(start))
		panicOnErr(err)
	}
}

func runEncoderV2(toc TOCV2) {
	file, _ := os.OpenFile(path.Join("example", "test_v2.yaml"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	defer file.Close()
	// refresh GC
	runtime.GC()
	start := time.Now()
	if *original {
		enc := yaml.NewEncoder(file)
		fmt.Println("Encoding slice into example/test_v2.yaml file")
		err := enc.Encode(toc)
		log.Printf("Time spent encoding: %s\n", time.Since(start))
		panicOnErr(err)
	} else {
		enc := gyaml.NewEncoder(file)
		fmt.Println("Encoding slice into example/test_v2.yaml file")
		err := enc.Encode(toc)
		log.Printf("Time spent encoding: %s\n", time.Since(start))
		panicOnErr(err)
	}
}

func runDecoder() {
	var toc TOC
	file, err := os.OpenFile(path.Join("example", "test.yaml"), os.O_RDONLY, 0644)
	if err != nil {
		log.Fatal("error opening test.yaml file with sample data, please run encode task first")
	}
	defer file.Close()
	// Force GC to ensure stats are fresh
	runtime.GC()
	start := time.Now()
	if *original {
		dec := yaml.NewDecoder(file)
		fmt.Println("Processing data from example/test.yaml file")
		err := dec.Decode(&toc)
		log.Printf("Time spent decoding: %s\n", time.Since(start))
		panicOnErr(err)
	} else {
		dec := gyaml.NewDecoder(file)
		fmt.Println("Reading data from example/test.yaml file")
		err := dec.Decode(&toc)
		log.Printf("Time spent decoding: %s\n", time.Since(start))
		panicOnErr(err)
	}

	//though all tests should be green make a naive check
	if len(toc.StatisticsEntries) != size {
		log.Printf("error: expected slice size %d, got %d\n", size, len(toc.StatisticsEntries))
	}
	idx := rand.Intn(size)
	expected := MetadataEntry{
		Name:  fmt.Sprintf("Name %d", idx),
		Value: fmt.Sprintf("Value %d", idx),
		Other: "",
	}
	if toc.StatisticsEntries[idx] != expected {
		log.Printf("error: expected %d record [%v], got [%v]\n", idx, expected, toc.StatisticsEntries[idx])
	}
}

func runDecoderV2() {
	var toc TOCV2
	file, err := os.OpenFile(path.Join("example", "test_v2.yaml"), os.O_RDONLY, 0644)
	if err != nil {
		log.Fatal("error opening test_v2.yaml file with sample data, please run encode task first")
	}
	defer file.Close()
	// Force GC to ensure stats are fresh
	runtime.GC()
	start := time.Now()
	if *original {
		dec := yaml.NewDecoder(file)
		fmt.Println("Processing data from example/test_v2.yaml file")
		err := dec.Decode(&toc)
		log.Printf("Time spent decoding: %s\n", time.Since(start))
		panicOnErr(err)
	} else {
		dec := gyaml.NewDecoder(file)
		fmt.Println("Reading data from example/test_v2.yaml file")
		err := dec.Decode(&toc)
		log.Printf("Time spent decoding: %s\n", time.Since(start))
		panicOnErr(err)
	}

	//though all tests should be green make a naive check
	if len(toc.StatisticsEntries) != size {
		log.Printf("error: expected slice size %d, got %d\n", size, len(toc.StatisticsEntries))
	}
}

func main() {
	var toc TOC
	var tocv2 TOCV2
	if !*decode {
		if *v2 {
			tocv2 = generateSliceV2()
		} else {
			toc = generateSlice()
		}
	}
	// profiling
	if *prof {
		f, err := os.Create("mem.prof")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		defer func() {
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Fatal("could not write memory profile: ", err)
			}
		}()

		fcpu, err := os.Create("cpu.prof")
		if err != nil {
			log.Fatal(err)
		}
		defer fcpu.Close()
		if err := pprof.StartCPUProfile(fcpu); err != nil {
			log.Fatal("Could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}
	var m runtime.MemStats

	if *decode {
		if *v2 {
			runDecoderV2()
		} else {
			runDecoder()
		}
	} else {
		if *v2 {
			runEncoderV2(tocv2)
		} else {
			runEncoder(toc)
		}
	}

	runtime.ReadMemStats(&m)
	fmt.Printf("Heap Used: %v KB\n", m.HeapAlloc/1024)
	fmt.Printf("Total Alloc: %v KB\n", m.TotalAlloc/1024)
	fmt.Printf("Sys: %v KB\n", m.Sys/1024)
	fmt.Printf("Number of GC runs: %v\n", m.NumGC)
}
