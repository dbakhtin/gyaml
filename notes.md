# Differencies from original go-yaml
* Float number 'e' notation
* Default struct names are printed as is, not lowercased
* Float 1.0 is encoded as 1
* Float 1e-07 is encoded as 1e-7
* Does not support math.Inf, math.NaN
* Does not support map[float64]any, map[bool]any


# Restart again
* Move slowly from simple cases to complex
* Constuctors: (Marshal (with indent ofc) with basic defaults), Encoder with Options (see v2 json pattern) (flow, quote styles options also)
* Calculate indent correctly. Take encoders pattern from json & adapt/replace code from yaml encoders
* Learn buffer basics, grow, etc
* Copy tests from go-yaml one by one. Basic tests for all types of data first + nesting. Border cases last