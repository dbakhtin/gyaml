# Differencies from original go-yaml
* No MarhshalWithOptions, encoder declaration is closer to encoding/json. So use NewEncoder(...).WithOption for advanced cases.
* No custom marshalers map, implement MarshalYAML for field types & MarshalText for map fields similar to encoding/json
* Smart anchor is enabled for structs by default. That means if encoder sees an anchor name collision it adds a unique number suffix.
* No smart anchors for maps
* No MapSlice data type
* No JSON marshaler
* No yaml comments
* Embedded structs are inlined by default with same priority shadowing as in encoding/json
* OmitZero tag for structs works like in encoding/json - struct is considered zero value if all (including unexported) fields are zero values. Can be dodged by implementing IsZeroer on struct or enhanced later if really needed.
* OmitEmpty works like in encoding/json, so it does not check nested struct types for emptyness at all (only scalar struct types, slices, maps, etc) :) Can mix with omitzero if requested, but this is a clear distinction between omitzero and omitempty and I thinks it is good as is.
* Marshal does not terminate result with \n, Encoder does. This is similar to encoder/json behavior.
* Faster encoding upto 10-14 times than go-yaml ymmv, see example
* Less gc pressure & memory consumption upto 10-14 times ymmv, same example

# Lint & vet
* Hotkey for GoLint current file

# Scaner
* Test some real examples

# TODO
* Fix package documentation
* Polish cmd/main.go memory & performance test for readability

# Encoder

# Decoder
* Start xD
* folded style multiline?
* single-quoted strings dont treat escape chars \t \n etc unlike double-quoted
* decode_test.go from go-yaml 
* fuzz_test.go
* yaml_test.go
