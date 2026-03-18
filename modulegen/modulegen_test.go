package modulegen

import "testing"

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
