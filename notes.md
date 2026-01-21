# Differencies from original go-yaml
* No MarhshalWithOptions, encoder declaration is closer to encoding/json. So use NewEncoder(...).WithOption for advanced cases.
* No custom marshalers map, implement MarshalYAML for field types & MarshalText for map fields similar to encoding/json
* Smart anchor is enabled for structs by default. That means if encoder sees an anchor name collision it adds a unique number suffix.
* No smart anchors for maps
* No MapSlice data type
* No JSON marshaler
* No yaml comments
* Embedded structs are inlined by default with same priority shadowing as in encoding/json
* Faster upto 6 times than go-yaml ymmv, see example
* Less gc pressure & memory consumption upto 10 times ymmv, same example

# TODO
* replace encoderOptions with a bitmask, move all map caches to encoderState
* Optimize marked with //TODO: functions & algorithms
* Commented tests with isEmpty for structs
* Add options to Marshal func - can help with custom MarshalYAMLing, ex: global flow tag
* Fix package documentation


# Decoder
* fuzz_test.go
* yaml_test.go
