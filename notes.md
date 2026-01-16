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
* Move valueIsStruct & other checks to params with flag
* Optimize marked with //TODO: functions & algorithms
* Optimize options & argument options
* Commented tests with isEmpty for structs

# Decoder
* fuzz_test.go
* yaml_test.go

# Printing indents
* Dont forget to commit before trying new features!!!
* Consider implementing bitmap options. And move encoder.inSlice & such to bitmap options that are passed as parameter to encoder (opts argument)
