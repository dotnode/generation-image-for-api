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
	"testing"
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
