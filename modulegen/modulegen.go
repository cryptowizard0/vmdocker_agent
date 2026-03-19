package modulegen

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	arSchema "github.com/permadao/goar/schema"
)

const (
	DefaultRuntimeBackend      = "sandbox"
	DefaultSandboxAgent        = "shell"
	DefaultOpenclawVersion     = "2026.3.1-beta.1"
	ModuleFormat               = "hymx.vmdocker.v0.0.1"
	ImageSourceTag             = "Image-Source"
	ImageArchiveTag            = "Image-Archive-Format"
	ImageSourceModuleData      = "module-data"
	ImageArchiveDockerSaveGzip = "docker-save+gzip"
)

type ModuleArtifact struct {
	ModuleBytes []byte
	Tags        []arSchema.Tag
}

type localImage struct {
	Name string
	ID   string
}

var progressWriter io.Writer = os.Stdout

func GenerateModuleArtifact() (ModuleArtifact, error) {
	base := []arSchema.Tag{
		{Name: "Runtime-Backend", Value: DefaultRuntimeBackend},
		{Name: "Sandbox-Agent", Value: DefaultSandboxAgent},
		{Name: "Openclaw-Version", Value: DefaultOpenclawVersion},
	}

	logProgressf("start generating module artifact")
	image, err := prepareFinalImage(context.Background())
	if err != nil {
		return ModuleArtifact{}, err
	}
	logProgressf("final image ready: name=%s id=%s", image.Name, image.ID)

	logProgressf("exporting docker image archive from %s", image.Name)
	moduleBytes, err := exportImageArchive(context.Background(), image.Name)
	if err != nil {
		return ModuleArtifact{}, err
	}
	logProgressf("image archive ready: compressed_size=%s", formatBytes(int64(len(moduleBytes))))

	tags := append(base,
		arSchema.Tag{Name: "Image-Name", Value: image.Name},
		arSchema.Tag{Name: "Image-ID", Value: image.ID},
		arSchema.Tag{Name: ImageSourceTag, Value: ImageSourceModuleData},
		arSchema.Tag{Name: ImageArchiveTag, Value: ImageArchiveDockerSaveGzip},
	)

	return ModuleArtifact{
		ModuleBytes: moduleBytes,
		Tags:        tags,
	}, nil
}

func prepareFinalImage(ctx context.Context) (localImage, error) {
	if os.Getenv("VMDOCKER_BUILD_DOCKERFILE") != "" || os.Getenv("VMDOCKER_BUILD_DOCKERFILE_PATH") != "" {
		logProgressf("generation mode: build")
		return buildModeImage(ctx)
	}
	logProgressf("generation mode: pull")
	return pullModeImage(ctx)
}

func buildModeImage(ctx context.Context) (localImage, error) {
	content, _, localDockerfilePath, err := resolveDockerfileContent()
	if err != nil {
		return localImage{}, fmt.Errorf("resolve Dockerfile content: %w", err)
	}

	contextRef := os.Getenv("VMDOCKER_BUILD_CONTEXT_URL")
	if contextRef == "" {
		if localDockerfilePath == "" {
			return localImage{}, fmt.Errorf("VMDOCKER_BUILD_CONTEXT_URL is required when using VMDOCKER_BUILD_DOCKERFILE_PATH without VMDOCKER_BUILD_DOCKERFILE")
		}
		contextRef = GetEnvWith("VMDOCKER_BUILD_CONTEXT_DIR", filepath.Dir(localDockerfilePath))
		contextRef, err = filepath.Abs(contextRef)
		if err != nil {
			return localImage{}, fmt.Errorf("resolve build context path %s: %w", contextRef, err)
		}
	}

	buildArgs := BuildArgsFromEnvMap()
	buildTag := GetEnvWith("VMDOCKER_BUILD_TAG", defaultBuildTag(string(content), contextRef, buildArgs))
	logProgressf("build image: tag=%s context=%s build_args=%d", buildTag, contextRef, len(buildArgs))
	if err := dockerBuild(ctx, string(content), contextRef, buildTag, buildArgs); err != nil {
		return localImage{}, err
	}

	imageID, err := inspectImageID(ctx, buildTag)
	if err != nil {
		return localImage{}, err
	}
	return localImage{Name: buildTag, ID: imageID}, nil
}

func pullModeImage(ctx context.Context) (localImage, error) {
	imageName := os.Getenv("VMDOCKER_SANDBOX_IMAGE_NAME")
	expectedID := os.Getenv("VMDOCKER_SANDBOX_IMAGE_ID")
	if imageName == "" {
		imageName = "chriswebber/docker-openclaw-sandbox:fix-test"
	}
	if expectedID == "" && imageName == "chriswebber/docker-openclaw-sandbox:fix-test" {
		expectedID = "sha256:4daa6b51a12f41566bca09c2ca92a4982263db47f40d20d11c8f83f6ae85bc0e"
	}

	logProgressf("check local image: name=%s", imageName)
	imageID, err := inspectImageID(ctx, imageName)
	if err != nil {
		logProgressf("local image missing, pulling %s", imageName)
		if err := dockerPull(ctx, imageName); err != nil {
			return localImage{}, err
		}
		imageID, err = inspectImageID(ctx, imageName)
		if err != nil {
			return localImage{}, err
		}
	}
	logProgressf("local image available: name=%s id=%s", imageName, imageID)
	if expectedID != "" && imageID != expectedID {
		return localImage{}, fmt.Errorf("local image id mismatch for %s: expected %s got %s", imageName, expectedID, imageID)
	}
	return localImage{Name: imageName, ID: imageID}, nil
}

func resolveDockerfileContent() ([]byte, string, string, error) {
	if dockerfilePath := os.Getenv("VMDOCKER_BUILD_DOCKERFILE"); dockerfilePath != "" {
		absPath, err := filepath.Abs(dockerfilePath)
		if err != nil {
			return nil, "", "", fmt.Errorf("resolve local Dockerfile path %s: %w", dockerfilePath, err)
		}
		content, err := os.ReadFile(absPath)
		if err != nil {
			return nil, "", "", fmt.Errorf("read local Dockerfile %s: %w", absPath, err)
		}
		return content, absPath, absPath, nil
	}

	dockerfilePath := os.Getenv("VMDOCKER_BUILD_DOCKERFILE_PATH")
	if dockerfilePath == "" {
		return nil, "", "", fmt.Errorf("set VMDOCKER_BUILD_DOCKERFILE or VMDOCKER_BUILD_DOCKERFILE_PATH")
	}
	contextURL := os.Getenv("VMDOCKER_BUILD_CONTEXT_URL")
	if contextURL == "" {
		return nil, "", "", fmt.Errorf("VMDOCKER_BUILD_CONTEXT_URL is required when using VMDOCKER_BUILD_DOCKERFILE_PATH")
	}

	rawURL, err := githubRawURLFromContextURL(contextURL, dockerfilePath)
	if err != nil {
		return nil, "", "", err
	}
	content, err := fetchURL(rawURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("fetch remote Dockerfile %s: %w", rawURL, err)
	}
	return content, rawURL, "", nil
}

func fetchURL(rawURL string) ([]byte, error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

var githubArchivePattern = regexp.MustCompile(`^/([^/]+)/([^/]+)/archive/(.+)\.(?:tar\.gz|zip)$`)

func githubRawURLFromContextURL(contextURL, dockerfilePath string) (string, error) {
	u, err := neturl.Parse(contextURL)
	if err != nil {
		return "", fmt.Errorf("parse VMDOCKER_BUILD_CONTEXT_URL %q: %w", contextURL, err)
	}
	if !strings.EqualFold(u.Host, "github.com") {
		return "", fmt.Errorf("remote Dockerfile fetch currently supports github.com context URLs only")
	}
	ref := u.Fragment
	path := strings.TrimSuffix(u.Path, "/")
	path = strings.TrimPrefix(path, "/")

	switch {
	case strings.HasSuffix(path, ".git"):
		path = strings.TrimSuffix(path, ".git")
		if ref == "" {
			ref = "HEAD"
		}
	case githubArchivePattern.MatchString(u.Path):
		matches := githubArchivePattern.FindStringSubmatch(u.Path)
		path = matches[1] + "/" + matches[2]
		if ref == "" {
			ref = strings.TrimPrefix(matches[3], "refs/heads/")
			ref = strings.TrimPrefix(ref, "refs/tags/")
		}
	default:
		return "", fmt.Errorf("unsupported github context URL format %q", contextURL)
	}

	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("unsupported github repo path %q", path)
	}
	dockerfilePath = strings.TrimPrefix(dockerfilePath, "/")
	if dockerfilePath == "" {
		return "", fmt.Errorf("VMDOCKER_BUILD_DOCKERFILE_PATH cannot be empty")
	}
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", parts[0], parts[1], ref, dockerfilePath), nil
}

func BuildArgsFromEnvMap() map[string]string {
	const prefix = "VMDOCKER_BUILD_ARG_"
	keys := make([]string, 0)
	values := make(map[string]string)
	for _, env := range os.Environ() {
		key, value, ok := strings.Cut(env, "=")
		if !ok || !strings.HasPrefix(key, prefix) {
			continue
		}
		argName := strings.TrimPrefix(key, prefix)
		if argName == "" {
			continue
		}
		keys = append(keys, argName)
		values[argName] = value
	}
	sort.Strings(keys)

	ordered := make(map[string]string, len(keys))
	for _, key := range keys {
		ordered[key] = values[key]
	}
	return ordered
}

func defaultBuildTag(dockerfile, contextRef string, buildArgs map[string]string) string {
	sum := sha256.New()
	sum.Write([]byte(dockerfile))
	sum.Write([]byte{0})
	sum.Write([]byte(contextRef))
	sum.Write([]byte{0})
	for _, buildArg := range sortedBuildArgs(buildArgs) {
		sum.Write([]byte(buildArg))
		sum.Write([]byte{0})
	}
	return "vmdocker-openclaw:" + hex.EncodeToString(sum.Sum(nil))[:12]
}

func sortedBuildArgs(args map[string]string) []string {
	if len(args) == 0 {
		return nil
	}
	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	buildArgs := make([]string, 0, len(keys))
	for _, key := range keys {
		buildArgs = append(buildArgs, key+"="+args[key])
	}
	return buildArgs
}

func dockerBuild(ctx context.Context, dockerfile, contextRef, tag string, buildArgs map[string]string) error {
	cliBin, err := dockerBinary()
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "vmdocker-module-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o600); err != nil {
		return err
	}

	args := []string{"build", "--progress=plain", "-f", dockerfilePath, "-t", tag}
	for _, buildArg := range sortedBuildArgs(buildArgs) {
		args = append(args, "--build-arg", buildArg)
	}
	for _, proxyKey := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"} {
		if val := os.Getenv(proxyKey); val != "" {
			val = strings.ReplaceAll(val, "127.0.0.1", "host.docker.internal")
			val = strings.ReplaceAll(val, "localhost", "host.docker.internal")
			args = append(args, "--build-arg", proxyKey+"="+val)
		}
	}
	args = append(args, contextRef)

	cmd := exec.CommandContext(ctx, cliBin, args...)
	cmd.Stdout = progressWriter
	cmd.Stderr = progressWriter
	logProgressf("docker build started: tag=%s", tag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed for %s: %w", tag, err)
	}
	logProgressf("docker build completed: tag=%s", tag)
	return nil
}

func dockerPull(ctx context.Context, imageName string) error {
	cliBin, err := dockerBinary()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, cliBin, "pull", imageName)
	cmd.Stdout = progressWriter
	cmd.Stderr = progressWriter
	logProgressf("docker pull started: image=%s", imageName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker pull failed for %s: %w", imageName, err)
	}
	logProgressf("docker pull completed: image=%s", imageName)
	return nil
}

func inspectImageID(ctx context.Context, imageName string) (string, error) {
	cliBin, err := dockerBinary()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, cliBin, "image", "inspect", "--format", "{{.Id}}", imageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("inspect image %s failed: %w\n%s", imageName, err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func exportImageArchive(ctx context.Context, imageName string) ([]byte, error) {
	cliBin, err := dockerBinary()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, cliBin, "save", imageName)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	progressReader := newArchiveProgressReader(stdout, imageName)
	if _, err := io.Copy(gz, progressReader); err != nil {
		_ = gz.Close()
		_ = cmd.Wait()
		return nil, fmt.Errorf("stream docker save output for %s failed: %w", imageName, err)
	}
	if err := gz.Close(); err != nil {
		_ = cmd.Wait()
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("docker save failed for %s: %w\n%s", imageName, err, stderr.String())
	}
	logProgressf("docker save completed: image=%s raw_size=%s compressed_size=%s", imageName, formatBytes(progressReader.total), formatBytes(int64(archive.Len())))
	return archive.Bytes(), nil
}

func dockerBinary() (string, error) {
	cliBin, err := exec.LookPath("docker")
	if err != nil {
		return "", fmt.Errorf("docker CLI is not available: %w", err)
	}
	return cliBin, nil
}

func GetEnvWith(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func logProgressf(format string, args ...any) {
	if progressWriter == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(progressWriter, "[module] %s %s\n", time.Now().Format("15:04:05"), msg)
}

type archiveProgressReader struct {
	reader     io.Reader
	imageName  string
	total      int64
	nextReport int64
}

func newArchiveProgressReader(reader io.Reader, imageName string) *archiveProgressReader {
	const reportEvery = 128 << 20
	return &archiveProgressReader{
		reader:     reader,
		imageName:  imageName,
		nextReport: reportEvery,
	}
}

func (r *archiveProgressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.total += int64(n)
		for r.total >= r.nextReport {
			logProgressf("docker save streaming: image=%s raw_size=%s", r.imageName, formatBytes(r.total))
			r.nextReport += 128 << 20
		}
	}
	return n, err
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
