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
* Replace && ns&inSlice == 0 with ns&inSlice == 0 || opts.indentSequence & check
* Test nested structs (>=2 levels)
* Move linebreak out of appendIndent? to struct/map/array encoders
* Optimize marked with //TODO: functions & algorithms
* Optimize options & argument options
* Commented tests with isEmpty for structs
* Encode slice of slices & compare with official yaml
* Check different indentNum options (<2, >2)
* Check test coverage & update tests (that null checks, etc)
* Write more tests with various kind of nesting (maps, structs, slices & again)


# Decoder
* fuzz_test.go
* yaml_test.go

# Printing indents
* Dont forget to commit before trying new features!!!
* Consider implementing bitmap options. And move encoder.inSlice & such to bitmap options that are passed as parameter to encoder (opts argument)

# Bitmap nesting
* Indent should care about linebreaks (and may be space after ':')
* Check every e.Write..() for printint correct indent
* Move some encoderOptions to e encoder
** Move level out of encoderOptions to function arguments instead of encoderOptions
* add a func appendString([]byte, []byte|string) []byte that adds a string to []byte and returns it
** replace all append(b, []byte(string)...) with appendString(b, string)?? or worthless??
** rewrite quoteWith, ... and multiline string encoder with []byte type? 
** add an indent function even for ' ' in case of scalar values
* add a func addPrefix([]byte, []byte|string) []byte for printing values with indent?
* add a func addSuffix([]byte|string, []byte) []byte also?
* Remove //+++ comment after all is done
920