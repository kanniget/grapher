package main

import (
	"fmt"
	"strconv"
	"strings"
)

// decodeTrapValue converts SNMP trap values represented either as dotted
// decimal addresses (e.g. "0.0.0.0") or as a list of bytes such as
// "[70 71 54]" into a human readable string.  Numeric arrays are treated as
// ASCII codes.
func decodeTrapValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		fields := strings.Fields(strings.Trim(raw, "[]"))
		buf := make([]byte, 0, len(fields))
		for _, f := range fields {
			v, err := strconv.Atoi(f)
			if err != nil {
				// fall back to original representation
				return raw
			}
			buf = append(buf, byte(v))
		}
		return string(buf)
	}
	return raw
}

// ExampleTrapDecoding demonstrates decoding of a typical FortiGate trap
// variable. The trap values are commonly provided by tools like snmptrapd in
// the format used in decodeTrapValue.
func ExampleTrapDecoding() {
	trap := "[70 71 54 72 49 70 84 66 50 50 57 48 49 52 48 53]"
	fmt.Println(decodeTrapValue(trap))
	// Output: FG6H1FTB22901405
}
