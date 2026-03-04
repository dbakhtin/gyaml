# Differencies from original go-yaml
* No MarhshalWithOptions, encoder declaration is closer to encoding/json. So use NewEncoder(...).WithOption for advanced cases.
* No custom marshalers map, implement MarshalYAML for field types & MarshalText for map fields similar to encoding/json
* Smart anchor is enabled for structs by default. That means if encoder sees an anchor name collision it adds a unique number suffix.
* No smart anchors for maps
* No MapSlice data type
* No JSON marshaler
* No yaml comments
* Embedded structs are inlined by default with same priority shadowing as in encoding/json
* "inline" tag is not supported, use embedding instead
* No AllowDuplicateMapKey option for decoder, duplicate keys are allowed by default. May be should change this.
* Struct fields implementing UnmarshalYAML(b []byte) interface do not support aliases to the outer scope
* OmitZero tag for structs works like in encoding/json - struct is considered zero value if all (including unexported) fields are zero values. Can be dodged by implementing IsZeroer on struct or enhanced later if really needed.
* OmitEmpty works like in encoding/json, so it does not check nested struct types for emptyness at all (only scalar struct types, slices, maps, etc) :) Can mix with omitzero if requested, but this is a clear distinction between omitzero and omitempty and I thinks it is good as is.
* Marshal does not terminate result with \n, Encoder does. This is similar to encoder/json behavior.
* Faster encoding upto 10-14 times than go-yaml ymmv, see example
* Less gc pressure & memory consumption upto 10-14 times ymmv, same example

# Lint & vet
* Hotkey for GoLint current file

# Scaner
* Test for "v: [a: b, c: d]". Probably need to split parseArray/parseObject consts for flow & block style
* Test more ~ null, can it have other values in same line? Or its always a stand alone value?
* Test for '\n' in in single-quoted strings
* Remove number states? Merge all into unquoted string
* Test some real examples

# TODO
* Fix package documentation
* Polish cmd/main.go memory & performance test for readability

# Nvim
* Hide types in Scopes (Locals) and possibly show them in dropdown or on hotkey
* Move linter hints/errors to the right

# Encoder
* Remove merge xD

# Decoder
* check go-yaml additional struct tags & decoder options
** remove destring? in object parsers
* do something with comments /remove?
* decode real examples
* examine func object() and use of objectInterface, arrayInterface()
* Compatibility with yaml 1.2:
** Documents in streams are now independent of each other, and no longer inherit preceding document directives if they do not define their own.
** Explicit document-end marker is now always required before a directive.
* Test anchors and alias type comparison when using d.objectInterface() & d.arrayInterface() or no anchors there at all?
* Create a new opcode scanBeginAnchor? and use it inside value or where quickScan is run and move code from quickScan to new function?
* Test multiline strings more with |-, |+, >, etc. And when it is not the last value in a map (so multiline state pops somewhere)
* Test & implement decoding into map[any]any, atm they all are replaced with map[string]any, see original go-yaml tests (map[interface{}]interface{})
** Use literalInterface & similar to determine type
* Test unicode \u in single quotes for proper escaping
* Do i need that looping possibility of the json decoder when it can read from reader in a cycle & decode each correct value?
* Test '''' is parsed as == "'" if can't then write in differences
* remove panics, use error codes?
** Test special chars in single/double quoted strings
* decode_test.go from go-yaml 
* fuzz_test.go
* yaml_test.go
