package modulegen

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cryptowizard0/vmdocker_agent/buildmanifest"
	arSchema "github.com/permadao/goar/schema"
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

func TestDefaultStartCommandUsesWorkspaceAssetRoot(t *testing.T) {
	required := []string{
		"VMDOCKER_RUNTIME_WORKSPACE",
		"VMDOCKER_AGENT_ASSET_ROOT",
		"VMDOCKER_AGENT_BUNDLE_ROOT",
		".vmdocker-agent",
		"bin/start-vmdocker-agent.sh",
	}
	for _, fragment := range required {
		if !strings.Contains(DefaultStartCommand, fragment) {
			t.Fatalf("DefaultStartCommand missing %q: %s", fragment, DefaultStartCommand)
		}
	}
	if strings.Contains(DefaultStartCommand, "/usr/local/bin/start-vmdocker-agent.sh") {
		t.Fatalf("DefaultStartCommand still uses the old /usr/local binary contract: %s", DefaultStartCommand)
	}
}

func TestManifestModuleTagsAreUsedAndImageTagsAreAdded(t *testing.T) {
	manifest := buildmanifest.Manifest{
		Name:           "claude",
		RuntimeProfile: "claude",
		Dockerfile:     "Dockerfile.claude",
		ImageName:      "example/claude:latest",
		ModuleTags:     map[string]string{"Start-Command": "workspace-start VMDOCKER_RUNTIME_WORKSPACE", "Sandbox-Agent": "shell"},
		StartCommand:   "workspace-start VMDOCKER_RUNTIME_WORKSPACE",
	}
	ops := moduleOps{
		inspectImageID: func(context.Context, string) (string, error) {
			return "sha256:abc", nil
		},
		buildImage: func(context.Context, buildmanifest.Manifest) error {
			t.Fatalf("buildImage should not be called when the image exists")
			return nil
		},
		exportImageArchive: func(context.Context, string) ([]byte, error) {
			return []byte("archive"), nil
		},
	}

	artifact, err := generateModuleArtifactFromManifest(context.Background(), manifest, ops)
	if err != nil {
		t.Fatalf("generateModuleArtifactFromManifest failed: %v", err)
	}
	tags := tagsToMap(artifact.Tags)
	if tags["Start-Command"] != "workspace-start VMDOCKER_RUNTIME_WORKSPACE" {
		t.Fatalf("Start-Command tag = %q", tags["Start-Command"])
	}
	if tags["Image-Name"] != "example/claude:latest" {
		t.Fatalf("Image-Name tag = %q", tags["Image-Name"])
	}
	if tags["Image-ID"] != "sha256:abc" {
		t.Fatalf("Image-ID tag = %q", tags["Image-ID"])
	}
	if tags[ImageSourceTag] != ImageSourceModuleData {
		t.Fatalf("Image-Source tag = %q", tags[ImageSourceTag])
	}
	if tags[ImageArchiveTag] != ImageArchiveDockerSaveGzip {
		t.Fatalf("Image-Archive-Format tag = %q", tags[ImageArchiveTag])
	}
}

func TestManifestGenerationBuildsWhenImageIsMissing(t *testing.T) {
	buildCalled := false
	manifest := buildmanifest.Manifest{
		Name:           "claude",
		RuntimeProfile: "claude",
		ImageName:      "example/claude:latest",
		Dockerfile:     "Dockerfile.claude",
		Context:        ".",
		ModuleTags:     map[string]string{"Start-Command": "workspace-start VMDOCKER_RUNTIME_WORKSPACE"},
		StartCommand:   "workspace-start VMDOCKER_RUNTIME_WORKSPACE",
	}
	ops := moduleOps{
		inspectImageID: func(context.Context, string) (string, error) {
			if buildCalled {
				return "sha256:after-build", nil
			}
			return "", errors.New("missing")
		},
		buildImage: func(_ context.Context, got buildmanifest.Manifest) error {
			buildCalled = true
			if got.ImageName != manifest.ImageName {
				t.Fatalf("build image name = %q", got.ImageName)
			}
			return nil
		},
		exportImageArchive: func(context.Context, string) ([]byte, error) {
			return []byte("archive"), nil
		},
	}

	artifact, err := generateModuleArtifactFromManifest(context.Background(), manifest, ops)
	if err != nil {
		t.Fatalf("generateModuleArtifactFromManifest failed: %v", err)
	}
	if !buildCalled {
		t.Fatalf("expected missing image to trigger build")
	}
	if got := tagsToMap(artifact.Tags)["Image-ID"]; got != "sha256:after-build" {
		t.Fatalf("Image-ID = %q", got)
	}
}

func TestResolveBuildContextsExpandsEnv(t *testing.T) {
	extraSource := t.TempDir()
	t.Setenv("EXTRA_CONTEXT_PATH", extraSource)

	got, err := resolveBuildContexts(buildmanifest.Manifest{
		BuildContexts: map[string]string{"extra_src": "${EXTRA_CONTEXT_PATH}"},
	})
	if err != nil {
		t.Fatalf("resolveBuildContexts failed: %v", err)
	}
	if got["extra_src"] != extraSource {
		t.Fatalf("extra_src context = %q, want %q", got["extra_src"], extraSource)
	}
}

func TestResolveBuildContextsRejectsMissingEnv(t *testing.T) {
	t.Setenv("EXTRA_CONTEXT_PATH", "")
	_, err := resolveBuildContexts(buildmanifest.Manifest{
		BuildContexts: map[string]string{"extra_src": "${EXTRA_CONTEXT_PATH}"},
	})
	if err == nil || !strings.Contains(err.Error(), "EXTRA_CONTEXT_PATH") {
		t.Fatalf("expected EXTRA_CONTEXT_PATH error, got %v", err)
	}
}

func TestDockerBuildArgsIncludeNamedBuildContexts(t *testing.T) {
	args := dockerBuildArgs("/tmp/Dockerfile", "example/hermes:latest", nil, map[string]string{
		"extra_src": "/tmp/extra-src",
	}, ".")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--build-context extra_src=/tmp/extra-src") {
		t.Fatalf("expected build context arg, got %v", args)
	}
}

func TestDockerBuildArgsIncludeGithubTokenSecret(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret")
	t.Setenv("GH_TOKEN", "")

	args := dockerBuildArgs("/tmp/Dockerfile", "example/hermes:latest", nil, nil, ".")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--secret id=github_token,env=GITHUB_TOKEN") {
		t.Fatalf("expected github token secret arg, got %v", args)
	}
}

func TestDockerBuildSecretEnvUsesGhTokenFallback(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "secret")

	got := dockerBuildSecretEnv()
	if len(got) != 1 || got[0] != "GITHUB_TOKEN=secret" {
		t.Fatalf("dockerBuildSecretEnv = %v", got)
	}
}

func TestLegacyBaseTagsStillIncludeDefaultStartCommand(t *testing.T) {
	tags := tagsToMap(baseModuleTags())
	if tags["Start-Command"] != DefaultStartCommand {
		t.Fatalf("legacy Start-Command = %q", tags["Start-Command"])
	}
	if tags["Openclaw-Version"] != DefaultOpenclawVersion {
		t.Fatalf("legacy Openclaw-Version = %q", tags["Openclaw-Version"])
	}
}

func tagsToMap(tags []arSchema.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		out[tag.Name] = tag.Value
	}
	return out
}
