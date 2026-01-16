package main

import (
	"flag"
	"fmt"
	"log"
	"os"
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
	for i := 0; i < size; i++ {
		name := fmt.Sprintf("Name %v", i)
		value := fmt.Sprintf("Value %v", i)
		toc.StatisticsEntries = append(toc.StatisticsEntries, MetadataEntry{Name: name, Value: value})
	}

	//profiling
	// f, err := os.Create("mem.prof")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer f.Close()

	file, _ := os.OpenFile("test.yaml", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	defer file.Close()
	start := time.Now()
	if *useOriginal {
		enc := yaml.NewEncoder(file)
		//defer enc.Close()
		fmt.Println("Serializing our slice to test.yaml file")
		err := enc.Encode(toc)
		log.Printf("Time spent serializing: %s\n", time.Since(start))
		panicOnErr(err)
		return
	}
	enc := gyaml.NewEncoder(file)
	//defer enc.Close()
	fmt.Println("Serializing our slice to test.yaml file")
	err := enc.Encode(toc)
	log.Printf("Time spent serializing: %s\n", time.Since(start))
	panicOnErr(err)

	// if err := pprof.WriteHeapProfile(f); err != nil {
	// 	log.Fatal("could not write memory profile: ", err)
	// }
}
