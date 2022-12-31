package main

import "testing"

func TestSimplifyTitle(t *testing.T) {
	simple := simplifyTitle("Scott Hanselman's Computer Zen")
	if simple != "scotthanselmanscomputerzen" {
		t.Fatalf("blargh %s", simple)
	}
}
