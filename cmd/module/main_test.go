package main

import "testing"

func TestResolveBuildProfilePath(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{name: "profile name", arg: "claude", want: "build/profiles/claude.toml"},
		{name: "hermes profile name", arg: "hermes", want: "build/profiles/hermes.toml"},
		{name: "toml path", arg: "build/profiles/claude.toml", want: "build/profiles/claude.toml"},
		{name: "nested path without extension", arg: "custom/claude", want: "custom/claude"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveBuildProfilePath(tt.arg); got != tt.want {
				t.Fatalf("resolveBuildProfilePath(%q) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}
