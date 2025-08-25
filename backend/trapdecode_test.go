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

func TestRenderTrap(t *testing.T) {
	vars := []trapVar{
		{OID: ".1.3.6.1.2.1.1.3.0", Value: "57587075"},
		{OID: ".1.3.6.1.4.1.12356.100.1.1.1.0", Value: "[70 71 54 72 49 70 84 66 50 50 57 48 49 52 48 53]"},
	}
	got, err := renderTrap("172.18.255.67", vars)
	if err != nil {
		t.Fatalf("renderTrap returned error: %v", err)
	}
	want := "172.18.255.67:\n  .1.3.6.1.2.1.1.3.0 => 57587075\n  .1.3.6.1.4.1.12356.100.1.1.1.0 => FG6H1FTB22901405\n"
	if got != want {
		t.Fatalf("unexpected render output:\n%s\nwant:\n%s", got, want)
	}
}
