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

# TIP OF THE DAY
* Use 'gc' to un/comment region

# Lint & vet
* Hotkey for GoLint current file

# Scaner
* BIG TODO: need spaceState []int to track spaces on each nesting. Without this I can't pop 2+ levels
* BIG TODO: add ObjectValueEmpty parse state (for parseState) to track if I have met a value. The difference with ObjectValue will help to track errors
* BIG TODO: create separate state values for flow style this will make code more clean
    when I have already parsed value and see another one like in a failing test. But commit before or create branch
* need more tests with objects in arrays, arrays in objects, arrays of arrays, etc
* in stateEndValue when state = ObjectValue implement the ',' logic for a \n
* Start from beginning, don't use json code, but compare if needed
* I have no clear criteria of stateEndValue, because no closing brackets, etc
** This means I need to reconsider checks inside eof()
* Count indents
* Entry point should be stateNewLine
** Inc space counter if space
** When string/number/unqstr met push parseState <- scalar (add to consts)
*** When ':' met check if next is space. if yes then substitute scalar with objectKey. If not scalar then error. If not space then continue str
    the end of the unquoted string is only a ": "
**** ": " -> replace objectKey with objectValue and parse value. Good to check if scalar contains '\n' then it's error, but can leave for later tuning.
***** parse value. if ': ' met in line then error. if '\n' met then
****** save last indent, reset current and inc space count. if indents equal then replace objectValue with objectKey and repeat above. If not then error 
        with one exception (later) if ":   \n" then objectValue was empty and expect a greater indent on new line with object value and increase nestedlevel (push).
        if indent decreased then pop or error.
**** eof -> only scalar in parseState, pop it and finish OK
* test with empty lines consisting of spaces + \n inside an object or array
* test with ": \n" for object keys

* Booleans
** remove? scanner does not care about the difference between bools & unquoted strings
* Slices
** count indents. if "- " met then begins array (push) and parse value till '\n' then stateNewLine
** count indents. If less than prev then pop. if greater then error. if "- " met and same indent then parse value till \n then stateNewLine. if not met then error.
** if eof() pop state & and check again (test this with multiple nesting)


# TODO
* Go TDD way: write red test -> fix source, enhance test -> fix source and so on.
* Decoder scanner
** Add state for unquoted string: it throws error on space met and is an alternative branch when scanning bools or unquoted chars
* Clear api surface (see: options, etc) and compare to encoding/json api surface
* Fix package documentation
* Polish cmd/main.go memory & performance test for readability

# Decoder
* fuzz_test.go
* yaml_test.go
