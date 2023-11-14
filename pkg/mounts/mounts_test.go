package mounts

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestMounts(t *testing.T) {
	m, warnings, err := Mounts()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	buf, _ := json.Marshal(m)

	for _, mm := range m {
		if mm.Mountpoint == "/System/Volumes/Update/mnt1" {
			t.Log(mm.Used)
		}
	}

	t.Logf("m: %v", string(buf))
	t.Logf("warnings: %v", warnings)
	t.Logf("err: %v", err)
}
