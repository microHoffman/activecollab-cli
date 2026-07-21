package main

import (
	"runtime/debug"
	"testing"
)

func TestSelectVersion(t *testing.T) {
	tests := []struct {
		name     string
		injected string
		info     *debug.BuildInfo
		ok       bool
		want     string
	}{
		{name: "linker version wins", injected: "1.2.3", info: &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}, ok: true, want: "1.2.3"},
		{name: "normalize linker version", injected: "v1.2.3", ok: false, want: "1.2.3"},
		{name: "go install module version", injected: "dev", info: &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, ok: true, want: "0.1.0"},
		{name: "local source build", injected: "dev", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, ok: true, want: "dev"},
		{name: "missing build info", injected: "dev", ok: false, want: "dev"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := selectVersion(test.injected, test.info, test.ok); got != test.want {
				t.Fatalf("selectVersion() = %q, want %q", got, test.want)
			}
		})
	}
}
