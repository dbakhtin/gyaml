package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/denisbakhtin/gyaml"
	"github.com/goccy/go-yaml"
)

var (
	useOriginal = flag.Bool("original", false, "use original encoder")
)

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

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	toc := TOC{}
	size := 1000000
	fmt.Printf("Preparing the slice of %d records\n", size)
	for i := range size {
		name := fmt.Sprintf("Name %v", i)
		value := fmt.Sprintf("Value %v", i)
		toc.StatisticsEntries = append(toc.StatisticsEntries, MetadataEntry{Name: name, Value: value})
	}

	// profiling
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

	file, _ := os.OpenFile("test.yaml", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	defer file.Close()
	var m runtime.MemStats
	// Force GC to ensure stats are fresh, if needed for immediate observation
	runtime.GC()
	start := time.Now()
	if *useOriginal {
		enc := yaml.NewEncoder(file)
		//defer enc.Close()
		fmt.Println("Serializing our slice to test.yaml file")
		err := enc.Encode(toc)
		log.Printf("Time spent serializing: %s\n", time.Since(start))
		panicOnErr(err)
	} else {
		enc := gyaml.NewEncoder(file)
		//defer enc.Close()
		fmt.Println("Serializing our slice to test.yaml file")
		err := enc.Encode(toc)
		log.Printf("Time spent serializing: %s\n", time.Since(start))
		panicOnErr(err)
	}

	runtime.ReadMemStats(&m)
	fmt.Printf("Heap Used: %v KB\n", m.HeapAlloc/1024)
	fmt.Printf("Total Alloc: %v KB\n", m.TotalAlloc/1024)
	fmt.Printf("Sys: %v KB\n", m.Sys/1024)
	fmt.Printf("Number of GC runs: %v\n", m.NumGC)
}
