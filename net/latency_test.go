package net

import "testing"

var tests = []string{
	"140.82.121.4:443",
	"142.250.179.174:443",
}

func TestGetInterface(t *testing.T) {
	dev, err := getInterface("")
	if err != nil {
		t.Error(err)
	}

	t.Logf("Interface found: %s", dev.Name)
}

func TestSendPacket(t *testing.T) {
	dev, err := getInterface("")
	if err != nil {
		t.Error(err)
	}

	t.Logf("Sending packet on: %s", dev.Name)
	s := NewScanner(dev.Name)
	results, err := s.StartLatencyScan(tests)
	if err != nil {
		t.Error(err)
	}

	t.Logf("%v", results)
}
