package main

import "testing"

func TestDecodeTrapValue(t *testing.T) {
	got := decodeTrapValue("[70 71 54 72 49 70 84 66 50 50 57 48 49 52 48 53]")
	if got != "FG6H1FTB22901405" {
		t.Fatalf("unexpected decode result: %q", got)
	}
	if decodeTrapValue("0.0.0.0") != "0.0.0.0" {
		t.Fatalf("dotted values should be preserved")
	}
}
