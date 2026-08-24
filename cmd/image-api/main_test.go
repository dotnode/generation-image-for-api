package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func writeTestConfig(t *testing.T, baseURL string) string {
	t.Helper()
	home := t.TempDir()
	config := fmt.Sprintf(`model_provider = "test_provider"

[model_providers.test_provider]
name = "test"
base_url = %q
env_key = "TEST_IMAGE_API_KEY"
`, baseURL)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", home)
	t.Setenv("TEST_IMAGE_API_KEY", "secret-test-token")
	return home
}

func decodeResult(t *testing.T, buffer *bytes.Buffer) outputResult {
	t.Helper()
	var result outputResult
	if err := json.Unmarshal(buffer.Bytes(), &result); err != nil {
		t.Fatalf("decode result %q: %v", buffer.String(), err)
	}
	return result
}

func TestVersionDoesNotRequireCodexConfig(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("version failed (%d): %s", code, stderr.String())
	}
	result := decodeResult(t, &stdout)
	if !result.OK || result.Version != appVersion {
		t.Fatalf("unexpected version result: %#v", result)
	}
}

func TestVersionFileMatchesBinary(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) != appVersion {
		t.Fatalf("VERSION file %q does not match binary version %q", strings.TrimSpace(string(raw)), appVersion)
	}
}

func TestVersionRejectsOtherModes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version", "--check"}, &stdout, &stderr); code != 2 {
		t.Fatalf("expected argument failure, got %d", code)
	}
	result := decodeResult(t, &stderr)
	if result.OK || result.Version != appVersion {
		t.Fatalf("unexpected version conflict result: %#v", result)
	}
}

func TestLoadProviderAndResolveEnvironmentToken(t *testing.T) {
	home := writeTestConfig(t, "https://example.invalid/v1")
	selection, err := loadProvider()
	if err != nil {
		t.Fatal(err)
	}
	if selection.ID != "test_provider" || selection.Home != home {
		t.Fatalf("unexpected selection: %#v", selection)
	}
	token, source, err := resolveToken(selection.Provider, selection.Home)
	if err != nil {
		t.Fatal(err)
	}
	if token != "secret-test-token" || source != "environment variable TEST_IMAGE_API_KEY" {
		t.Fatalf("unexpected token resolution: token=%q source=%q", token, source)
	}
}

func TestGenerationRequestAndBase64Save(t *testing.T) {
	png := []byte("not-a-real-png-but-valid-test-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/models" {
			_ = json.NewEncoder(writer).Encode(map[string]any{"data": []map[string]string{{"id": "gpt-image-2"}, {"id": "text-model"}}})
			return
		}
		if request.URL.Path != "/v1/images/generations" || request.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer secret-test-token" {
			t.Error("missing bearer token")
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] != "gpt-image-2" || payload["prompt"] != "blue circle" || payload["quality"] != "low" {
			t.Errorf("unexpected payload: %#v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(imageResponse{Data: []imageItem{{Base64: base64.StdEncoding.EncodeToString(png)}}})
	}))
	defer server.Close()
	writeTestConfig(t, server.URL+"/v1")

	outputDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt", "blue circle", "--output-dir", outputDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed (%d): %s", code, stderr.String())
	}
	result := decodeResult(t, &stdout)
	if result.Operation != "generate" || result.Path == "" || result.Markdown == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	saved, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, png) {
		t.Fatalf("saved bytes differ: %q", saved)
	}
}

func TestMultipartEditWithRepeatedImagesAndMask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/models" {
			_ = json.NewEncoder(writer).Encode(map[string]any{"data": []map[string]string{{"id": "gpt-image-2"}}})
			return
		}
		if request.URL.Path != "/v1/images/edits" || request.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if err := request.ParseMultipartForm(2 * 1024 * 1024); err != nil {
			t.Fatal(err)
		}
		if request.FormValue("prompt") != "combine" || request.FormValue("size") != "512x512" {
			t.Errorf("unexpected fields: %#v", request.MultipartForm.Value)
		}
		if len(request.MultipartForm.File["image[]"]) != 2 {
			t.Fatalf("expected 2 images, got %#v", request.MultipartForm.File)
		}
		if len(request.MultipartForm.File["mask"]) != 1 {
			t.Fatalf("expected mask, got %#v", request.MultipartForm.File)
		}
		for _, field := range []string{"image[]", "mask"} {
			for _, header := range request.MultipartForm.File[field] {
				file, err := header.Open()
				if err != nil {
					t.Fatal(err)
				}
				content, _ := io.ReadAll(file)
				_ = file.Close()
				if len(content) == 0 {
					t.Errorf("empty upload for %s", field)
				}
			}
		}
		_ = json.NewEncoder(writer).Encode(imageResponse{Data: []imageItem{{Base64: base64.StdEncoding.EncodeToString([]byte("edited"))}}})
	}))
	defer server.Close()
	writeTestConfig(t, server.URL+"/v1")

	inputDir := t.TempDir()
	image1 := filepath.Join(inputDir, "one.png")
	image2 := filepath.Join(inputDir, "two.jpg")
	mask := filepath.Join(inputDir, "mask.png")
	for path, content := range map[string]string{image1: "one", image2: "two", mask: "mask"} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--edit", image1, "--edit", image2, "--mask", mask,
		"--prompt", "combine", "--size", "512x512", "--quality", "auto",
		"--output-dir", t.TempDir(),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run failed (%d): %s", code, stderr.String())
	}
	result := decodeResult(t, &stdout)
	if result.Operation != "edit" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestHTTPErrorRedactsToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"secret-test-token rejected"}`))
	}))
	defer server.Close()
	writeTestConfig(t, server.URL)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt", "test"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected failure")
	}
	if bytes.Contains(stderr.Bytes(), []byte("secret-test-token")) || !bytes.Contains(stderr.Bytes(), []byte("<redacted>")) {
		t.Fatalf("token was not redacted: %s", stderr.String())
	}
}

func TestOfficialProviderRoutesToCodexImagegen(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("model_provider = \"openai\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"do-not-read"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", home)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--check"}, &stdout, &stderr); code != 0 {
		t.Fatalf("check failed (%d): %s", code, stderr.String())
	}
	result := decodeResult(t, &stdout)
	if result.ProviderType != "official_openai" || result.ThirdParty == nil || *result.ThirdParty {
		t.Fatalf("unexpected official result: %#v", result)
	}
	if result.UseCodexImagegen == nil || !*result.UseCodexImagegen || result.AuthMode != "chatgpt" {
		t.Fatalf("missing official routing instruction: %#v", result)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--prompt", "official image"}, &stdout, &stderr); code != 3 {
		t.Fatalf("expected route exit code 3, got %d: %s", code, stderr.String())
	}
	result = decodeResult(t, &stderr)
	if result.OK || result.UseCodexImagegen == nil || !*result.UseCodexImagegen {
		t.Fatalf("unexpected official generation route: %#v", result)
	}
}

func TestThirdPartyWithoutImageModelsStopsBeforeGeneration(t *testing.T) {
	generationCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/models" {
			_ = json.NewEncoder(writer).Encode(map[string]any{"data": []map[string]string{{"id": "text-only-model"}}})
			return
		}
		generationCalled = true
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	writeTestConfig(t, server.URL+"/v1")

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--prompt", "must not be sent"}, &stdout, &stderr); code == 0 {
		t.Fatal("expected unsupported-image-model failure")
	}
	if generationCalled {
		t.Fatal("generation endpoint was called without image-model support")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("no recognizable image model")) {
		t.Fatalf("unexpected error: %s", stderr.String())
	}
}

func TestOfficialURLDetection(t *testing.T) {
	if !isOfficialOpenAIURL("https://api.openai.com/v1") {
		t.Fatal("official OpenAI URL was not detected")
	}
	if isOfficialOpenAIURL("https://api.openai.com.example.com/v1") {
		t.Fatal("lookalike URL was classified as official")
	}
}

func TestSyncRoutingEnablesCustomSkillForSupportedThirdParty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": []map[string]string{{"id": "gpt-image-2"}}})
	}))
	defer server.Close()
	home := writeTestConfig(t, server.URL+"/v1")
	configPath := filepath.Join(home, "config.toml")
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	legacyCustomPath := filepath.Join(home, "skills", "generation-image-for-api")
	customPath := filepath.Join(legacyCustomPath, "SKILL.md")
	config := string(original) + fmt.Sprintf(`
# preserve this comment
[[skills.config]]
path = %q
enabled = false
`, legacyCustomPath)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	original = []byte(config)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--sync-routing"}, &stdout, &stderr); code != 0 {
		t.Fatalf("sync failed (%d): %s", code, stderr.String())
	}
	result := decodeResult(t, &stdout)
	if result.RoutingChanged == nil || !*result.RoutingChanged || result.CustomSkillEnabled == nil || !*result.CustomSkillEnabled {
		t.Fatalf("unexpected sync result: %#v", result)
	}
	if result.SystemSkillEnabled == nil || *result.SystemSkillEnabled || result.BackupPath == "" {
		t.Fatalf("unexpected system routing result: %#v", result)
	}
	assertSkillEnabled(t, configPath, customPath, true)
	assertSkillNotConfigured(t, configPath, legacyCustomPath)
	assertSkillEnabled(t, configPath, filepath.Join(home, "skills", ".system", "imagegen", "SKILL.md"), false)
	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(updated, []byte("# preserve this comment")) {
		t.Fatal("unrelated config comment was not preserved")
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(backup, original) {
		t.Fatal("config backup does not match original")
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--sync-routing"}, &stdout, &stderr); code != 0 {
		t.Fatalf("second sync failed (%d): %s", code, stderr.String())
	}
	result = decodeResult(t, &stdout)
	if result.RoutingChanged == nil || *result.RoutingChanged || result.BackupPath != "" {
		t.Fatalf("sync was not idempotent: %#v", result)
	}
}

func TestSyncRoutingEnablesSystemSkillForOfficialProvider(t *testing.T) {
	home := t.TempDir()
	legacyCustomPath := filepath.Join(home, "skills", "generation-image-for-api")
	legacySystemPath := filepath.Join(home, "skills", ".system", "imagegen")
	customPath := filepath.Join(legacyCustomPath, "SKILL.md")
	systemPath := filepath.Join(legacySystemPath, "SKILL.md")
	config := fmt.Sprintf(`model_provider = "openai"

[[skills.config]]
path = %q
enabled = true

[[skills.config]]
path = %q
enabled = false
`, legacyCustomPath, legacySystemPath)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"auth_mode":"chatgpt"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", home)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--sync-routing"}, &stdout, &stderr); code != 0 {
		t.Fatalf("sync failed (%d): %s", code, stderr.String())
	}
	result := decodeResult(t, &stdout)
	if result.CustomSkillEnabled == nil || *result.CustomSkillEnabled {
		t.Fatalf("custom skill was not disabled: %#v", result)
	}
	if result.SystemSkillEnabled == nil || !*result.SystemSkillEnabled || result.UseCodexImagegen == nil || !*result.UseCodexImagegen {
		t.Fatalf("system skill was not enabled: %#v", result)
	}
	assertSkillEnabled(t, filepath.Join(home, "config.toml"), customPath, false)
	assertSkillEnabled(t, filepath.Join(home, "config.toml"), systemPath, true)
	assertSkillNotConfigured(t, filepath.Join(home, "config.toml"), legacyCustomPath)
	assertSkillNotConfigured(t, filepath.Join(home, "config.toml"), legacySystemPath)
}

func TestUpdateSkillConfigDeduplicatesLegacyAndCanonicalPaths(t *testing.T) {
	home := t.TempDir()
	legacyPath := filepath.Join(home, "skills", "generation-image-for-api")
	canonicalPath := filepath.Join(legacyPath, "SKILL.md")
	raw := []byte(fmt.Sprintf(`[[skills.config]]
path = %q
enabled = false

[[skills.config]]
path = %q
enabled = false
`, legacyPath, canonicalPath))
	desired := []skillToggle{{Path: canonicalPath, Aliases: []string{legacyPath}, Enabled: true}}

	updated, changed, err := updateSkillConfig(raw, home, desired)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected legacy skill entries to be migrated")
	}
	var document skillConfigDocument
	if err := toml.Unmarshal(updated, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Skills.Config) != 1 {
		t.Fatalf("expected one canonical skill entry, got %d", len(document.Skills.Config))
	}
	item := document.Skills.Config[0]
	if filepath.Clean(item.Path) != filepath.Clean(canonicalPath) || item.Enabled == nil || !*item.Enabled {
		t.Fatalf("unexpected canonical skill entry: %#v", item)
	}
}

func TestSyncRoutingLeavesConfigUntouchedWhenNoImageRouteExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": []map[string]string{{"id": "text-only"}}})
	}))
	defer server.Close()
	home := writeTestConfig(t, server.URL+"/v1")
	configPath := filepath.Join(home, "config.toml")
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--sync-routing"}, &stdout, &stderr); code == 0 {
		t.Fatal("expected routing failure")
	}
	current, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, original) {
		t.Fatal("config changed despite missing image route")
	}
	backups, err := filepath.Glob(configPath + ".image-routing-backup-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("unexpected backup after failed sync: %#v", backups)
	}
}

func TestSyncRoutingFallsBackToSystemSkillForChatGPTLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": []map[string]string{{"id": "text-only"}}})
	}))
	defer server.Close()
	home := writeTestConfig(t, server.URL+"/v1")
	if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"auth_mode":"chatgpt"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--sync-routing"}, &stdout, &stderr); code != 0 {
		t.Fatalf("sync failed (%d): %s", code, stderr.String())
	}
	result := decodeResult(t, &stdout)
	if result.OfficialLogin == nil || !*result.OfficialLogin {
		t.Fatalf("ChatGPT login was not detected: %#v", result)
	}
	if result.CustomSkillEnabled == nil || *result.CustomSkillEnabled {
		t.Fatalf("custom skill was not disabled: %#v", result)
	}
	if result.SystemSkillEnabled == nil || !*result.SystemSkillEnabled || result.UseCodexImagegen == nil || !*result.UseCodexImagegen {
		t.Fatalf("system fallback was not enabled: %#v", result)
	}
}

func assertSkillEnabled(t *testing.T, configPath, skillPath string, expected bool) {
	t.Helper()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var document skillConfigDocument
	if err := toml.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	for _, item := range document.Skills.Config {
		if filepath.Clean(item.Path) == filepath.Clean(skillPath) {
			if item.Enabled == nil || *item.Enabled != expected {
				t.Fatalf("skill %s enabled=%v, expected %v", skillPath, item.Enabled, expected)
			}
			return
		}
	}
	t.Fatalf("skill config not found: %s", skillPath)
}

func assertSkillNotConfigured(t *testing.T, configPath, skillPath string) {
	t.Helper()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var document skillConfigDocument
	if err := toml.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	for _, item := range document.Skills.Config {
		if filepath.Clean(item.Path) == filepath.Clean(skillPath) {
			t.Fatalf("legacy skill config still present: %s", skillPath)
		}
	}
}
