package main

import (
	"fmt"
	"os"

	"github.com/denisbakhtin/gyaml"
)

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
	enc := gyaml.NewEncoder(file)
	//defer enc.Close()
	fmt.Println("Serializing our slice to test.yaml file")
	err := enc.Encode(toc)
	panicOnErr(err)
	// if err := pprof.WriteHeapProfile(f); err != nil {
	// 	log.Fatal("could not write memory profile: ", err)
	// }
}
