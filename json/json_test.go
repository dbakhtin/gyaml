package json

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type animal struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func TestAnimalJson(t *testing.T) {
	a := animal{
		Type: "canine",
		Name: "cat",
	}
	json.NewEncoder(os.Stdout).Encode(&a)
}

func TestAnimalSliceJson(t *testing.T) {
	size := 1_000_000
	animals := make([]animal, 0, size)
	for i := range size {
		animals = append(animals, animal{Name: fmt.Sprintf("Cat %d", i+1), Type: fmt.Sprintf("Canine %d", i+1)})
	}
	json.NewEncoder(os.Stdout).Encode(animals)
}
