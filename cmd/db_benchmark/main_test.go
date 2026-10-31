package main

import "testing"

func TestParseLimits(t *testing.T) {
	limits, err := parseLimits("1, 10,100")
	if err != nil {
		t.Fatalf("parseLimits() error = %v", err)
	}
	if len(limits) != 3 || limits[1] != 10 {
		t.Fatalf("parseLimits() = %#v", limits)
	}
	if _, err := parseLimits("1,0"); err == nil {
		t.Fatal("parseLimits() accepted zero")
	}
}
