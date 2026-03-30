package modulegen

import (
	"testing"
)

func TestGithubRawURLFromContextURL(t *testing.T) {
	t.Run("git url with ref", func(t *testing.T) {
		got, err := githubRawURLFromContextURL("https://github.com/example/repo.git#main", "Dockerfile.sandbox")
		if err != nil {
			t.Fatalf("githubRawURLFromContextURL returned error: %v", err)
		}
		want := "https://raw.githubusercontent.com/example/repo/main/Dockerfile.sandbox"
		if got != want {
			t.Fatalf("unexpected raw url: got %q want %q", got, want)
		}
	})

	t.Run("archive url infers ref", func(t *testing.T) {
		got, err := githubRawURLFromContextURL("https://github.com/example/repo/archive/refs/heads/dev.tar.gz", "/docker/Dockerfile")
		if err != nil {
			t.Fatalf("githubRawURLFromContextURL returned error: %v", err)
		}
		want := "https://raw.githubusercontent.com/example/repo/dev/docker/Dockerfile"
		if got != want {
			t.Fatalf("unexpected raw url: got %q want %q", got, want)
		}
	})
}

func TestBuildArgsFromEnvMap(t *testing.T) {
	t.Setenv("VMDOCKER_BUILD_ARG_ZETA", "z")
	t.Setenv("VMDOCKER_BUILD_ARG_ALPHA", "a")

	got := sortedBuildArgs(BuildArgsFromEnvMap())
	want := []string{"ALPHA=a", "ZETA=z"}
	if len(got) != len(want) {
		t.Fatalf("unexpected build arg count: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected build arg at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestGenerateModuleArtifactBaseTagsDoNotIncludeRuntimeBackend(t *testing.T) {
	tags := baseModuleTags()
	want := map[string]string{
		"Sandbox-Agent":    DefaultSandboxAgent,
		"Openclaw-Version": DefaultOpenclawVersion,
		"Start-Command":    DefaultStartCommand,
	}

	if len(tags) != len(want) {
		t.Fatalf("unexpected base tag count: got %d want %d", len(tags), len(want))
	}

	for _, tag := range tags {
		if tag.Name == "Runtime-Backend" {
			t.Fatalf("Runtime-Backend should not be emitted in module tags")
		}
		if got, ok := want[tag.Name]; !ok {
			t.Fatalf("unexpected base tag %q", tag.Name)
		} else if tag.Value != got {
			t.Fatalf("unexpected value for %q: got %q want %q", tag.Name, tag.Value, got)
		}
		delete(want, tag.Name)
	}

	if len(want) != 0 {
		t.Fatalf("missing expected base tags: %v", want)
	}
}
