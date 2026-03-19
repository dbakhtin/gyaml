# YAML support for the Go language
## Introduction
The gyaml package enables Go programs to encode and decode YAML text. It was based on
[go-yaml](https://github.com/goccy/go-yaml) and standard `encoding/json` projects. The idea was to preserve encoding/json performance (as much as possible) and go-yaml's flexibility.

## Attention
This package is in its early stage, so errors may occur. Please file an issue with a brief example to help me debug and improve the code. Thank you.

## Performance comparison (see Makefile & cmd/main.go for a synthetic example):
* Encoding faster upto **12-15** times than go-yaml, 10-12 times less memory & gc pressure (`make encode` and `make encodev2` for a little bit more complex data structure)
* Decoding faster upto **7-9** times than go-yaml, 15-18 times less memory & gc pressure (`make decode` and `make decodev2`, run them only after encoders have created a corresponding test yaml file)

## Key differencies from original go-yaml
* No MarhshalWithOptions, encoder declaration is closer to encoding/json. So use NewEncoder(...).WithOption for advanced cases.
* No custom marshalers map, implement MarshalYAML for field types & MarshalText for map fields similar to encoding/json
* Smart anchor is enabled for structs by default. That means if encoder sees an anchor name collision it adds a unique number suffix.
* No smart anchors for maps
* No MapSlice data type
* No JSON marshaler
* No yaml comments
* Embedded structs are inlined by default with same priority shadowing as in encoding/json
* "inline" tag is not supported, use embedding instead
* Numbers are encoded and decoded without quotes, i.e "1" can't be deserialized into int (may add this in future if really needed)
* No AllowDuplicateMapKey option for decoder, duplicate keys are allowed by default (probably should change this in future).
* Struct fields implementing UnmarshalYAML(b []byte) interface do not support aliases to the outer scope
* OmitZero tag for structs works like in encoding/json - struct is considered zero value if all (including unexported) fields are zero values. Can be bypassed by implementing IsZeroer on struct (or fixed later if really needed).
* OmitEmpty works like in encoding/json, so it does not check nested struct types for emptiness (only scalar struct types, slices, maps, etc). So far this is a clear distinction between omitzero and omitempty and I thinks it is good as is.
* Marshal does not terminate result with \n, Encoder does. This is similar to encoder/json behavior.

## Compatibility
The gyaml package supports most of YAML 1.2 and does not support merging operator `<<` from YAML 1.1.

## TODO
* Decoding multi-dimensional block sequences "- - a", "- - - a" (encoding works already). For now use mixed "- [a, b]" or full flow [[a,b],..] styles.
* [a: b, c: d] syntax (an array with nested objects (key: value without {})).
* Improve performance.

## License
BSD-2 simplified license, see LICENSE file.
