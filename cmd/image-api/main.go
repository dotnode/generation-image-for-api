package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	userAgent   = "codex-generation-image-for-api-skill/2.0"
	maxEditSize = 50 * 1024 * 1024
)

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }
func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

type options struct {
	check      bool
	listModels bool
	prompt     string
	edits      stringList
	mask       string
	model      string
	size       string
	quality    string
	outputDir  string
	timeout    time.Duration
}

type providerAuth struct {
	Command   string   `toml:"command"`
	Args      []string `toml:"args"`
	CWD       string   `toml:"cwd"`
	TimeoutMS int      `toml:"timeout_ms"`
}

type providerConfig struct {
	Name                    string        `toml:"name"`
	BaseURL                 string        `toml:"base_url"`
	EnvKey                  string        `toml:"env_key"`
	RequiresOpenAIAuth      bool          `toml:"requires_openai_auth"`
	ExperimentalBearerToken string        `toml:"experimental_bearer_token"`
	Auth                    *providerAuth `toml:"auth"`
}

type codexConfig struct {
	ModelProvider  string                    `toml:"model_provider"`
	OpenAIBaseURL  string                    `toml:"openai_base_url"`
	ModelProviders map[string]providerConfig `toml:"model_providers"`
}

type providerSelection struct {
	ID       string
	Provider providerConfig
	Home     string
	Official bool
}

type imageResponse struct {
	Data         []imageItem `json:"data"`
	OutputFormat string      `json:"output_format"`
}

type imageItem struct {
	Base64 string `json:"b64_json"`
	URL    string `json:"url"`
}

type outputResult struct {
	OK                 bool     `json:"ok"`
	Provider           string   `json:"provider,omitempty"`
	ProviderType       string   `json:"provider_type,omitempty"`
	ThirdParty         *bool    `json:"third_party,omitempty"`
	AuthMode           string   `json:"auth_mode,omitempty"`
	BaseURL            string   `json:"base_url,omitempty"`
	GenerationEndpoint string   `json:"generation_endpoint,omitempty"`
	EditEndpoint       string   `json:"edit_endpoint,omitempty"`
	TokenSource        string   `json:"token_source,omitempty"`
	TokenPresent       bool     `json:"token_present,omitempty"`
	ImageModels        []string `json:"image_models,omitempty"`
	TotalModels        *int     `json:"total_models,omitempty"`
	ImageSupported     *bool    `json:"image_supported,omitempty"`
	UseCodexImagegen   *bool    `json:"use_codex_imagegen,omitempty"`
	Message            string   `json:"message,omitempty"`
	Model              string   `json:"model,omitempty"`
	Operation          string   `json:"operation,omitempty"`
	Path               string   `json:"path,omitempty"`
	Markdown           string   `json:"markdown,omitempty"`
	Error              string   `json:"error,omitempty"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil {
		writeJSON(stderr, outputResult{OK: false, Error: err.Error()})
		return 2
	}

	selection, err := loadProvider()
	if err != nil {
		writeJSON(stderr, outputResult{OK: false, Error: err.Error()})
		return 1
	}
	baseURL := strings.TrimRight(selection.Provider.BaseURL, "/")
	thirdParty := !selection.Official
	if selection.Official {
		useCodexImagegen := true
		message := "Official Codex/OpenAI provider detected. Use Codex's built-in image generation tool instead of this third-party API CLI."
		result := outputResult{
			OK: true, Provider: selection.ID, ProviderType: "official_openai",
			ThirdParty: &thirdParty, AuthMode: loadAuthMode(selection.Home), BaseURL: baseURL,
			UseCodexImagegen: &useCodexImagegen, Message: message,
		}
		if opts.check || opts.listModels {
			writeJSON(stdout, result)
			return 0
		}
		result.OK = false
		result.Error = message
		writeJSON(stderr, result)
		return 3
	}

	token, tokenSource, err := resolveToken(selection.Provider, selection.Home)
	if err != nil {
		writeJSON(stderr, outputResult{OK: false, Error: err.Error()})
		return 1
	}

	fail := func(err error) int {
		writeJSON(stderr, outputResult{OK: false, Error: redact(err.Error(), token)})
		return 1
	}

	client := &http.Client{Timeout: opts.timeout}
	var modelsResponse map[string]any
	if err := requestJSON(client, http.MethodGet, baseURL+"/models", token, nil, &modelsResponse); err != nil {
		return fail(fmt.Errorf("could not verify third-party image-model support: %w", err))
	}
	models, total := likelyImageModels(modelsResponse)
	imageSupported := len(models) > 0
	useCodexImagegen := false
	if opts.check || opts.listModels {
		writeJSON(stdout, outputResult{
			OK: true, Provider: selection.ID, ProviderType: "third_party_api",
			ThirdParty: &thirdParty, BaseURL: baseURL,
			GenerationEndpoint: baseURL + "/images/generations",
			EditEndpoint:       baseURL + "/images/edits",
			TokenSource:        tokenSource, TokenPresent: token != "",
			ImageModels: models, TotalModels: total, ImageSupported: &imageSupported,
			UseCodexImagegen: &useCodexImagegen,
		})
		return 0
	}
	if !imageSupported {
		return fail(errors.New("the selected third-party API exposes no recognizable image model in /models"))
	}
	if !containsString(models, opts.model) {
		return fail(fmt.Errorf("image model %q is not exposed by the selected third-party API; available image models: %s", opts.model, strings.Join(models, ", ")))
	}

	if strings.TrimSpace(opts.prompt) == "" {
		return fail(errors.New("--prompt is required for image generation"))
	}
	if len(opts.edits) > 5 {
		return fail(errors.New("at most 5 --edit images are supported per request"))
	}
	if opts.mask != "" && len(opts.edits) == 0 {
		return fail(errors.New("--mask requires at least one --edit image"))
	}

	var response imageResponse
	operation := "generate"
	if len(opts.edits) > 0 {
		operation = "edit"
		fields := map[string]string{
			"model": opts.model, "prompt": opts.prompt, "n": "1", "size": opts.size,
		}
		if opts.quality != "" && !strings.EqualFold(opts.quality, "auto") {
			fields["quality"] = opts.quality
		}
		fieldName := "image"
		if len(opts.edits) > 1 {
			fieldName = "image[]"
		}
		files := make([]uploadFile, 0, len(opts.edits)+1)
		for _, path := range opts.edits {
			files = append(files, uploadFile{Field: fieldName, Path: path})
		}
		if opts.mask != "" {
			files = append(files, uploadFile{Field: "mask", Path: opts.mask})
		}
		if err := requestMultipart(client, baseURL+"/images/edits", token, fields, files, &response); err != nil {
			return fail(err)
		}
	} else {
		payload := map[string]any{
			"model": opts.model, "prompt": opts.prompt, "n": 1, "size": opts.size,
		}
		if opts.quality != "" && !strings.EqualFold(opts.quality, "auto") {
			payload["quality"] = opts.quality
		}
		if err := requestJSON(client, http.MethodPost, baseURL+"/images/generations", token, payload, &response); err != nil {
			return fail(err)
		}
	}

	path, err := saveImage(client, response, opts.outputDir, operation)
	if err != nil {
		return fail(err)
	}
	writeJSON(stdout, outputResult{
		OK: true, Provider: selection.ID, ProviderType: "third_party_api",
		ThirdParty: &thirdParty, ImageSupported: &imageSupported,
		UseCodexImagegen: &useCodexImagegen, Model: opts.model, Operation: operation,
		Path: path, Markdown: "![Generated image](" + path + ")",
	})
	return 0
}

func parseOptions(args []string) (options, error) {
	var opts options
	defaultModel := os.Getenv("NEWAPI_IMAGE_MODEL")
	if defaultModel == "" {
		defaultModel = "gpt-image-2"
	}
	flags := flag.NewFlagSet("image-api", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&opts.check, "check", false, "validate provider and token resolution")
	flags.BoolVar(&opts.listModels, "list-models", false, "list likely image models")
	flags.StringVar(&opts.prompt, "prompt", "", "image prompt")
	flags.Var(&opts.edits, "edit", "edit/reference image path; repeat for multiple images")
	flags.StringVar(&opts.mask, "mask", "", "optional mask image")
	flags.StringVar(&opts.model, "model", defaultModel, "image model")
	flags.StringVar(&opts.size, "size", "1024x1024", "image size")
	flags.StringVar(&opts.quality, "quality", "low", "image quality")
	flags.StringVar(&opts.outputDir, "output-dir", filepath.FromSlash("output/generation-image-for-api"), "output directory")
	var timeoutSeconds int
	flags.IntVar(&timeoutSeconds, "timeout", 180, "HTTP timeout in seconds")
	if err := flags.Parse(args); err != nil {
		return opts, err
	}
	if flags.NArg() != 0 {
		return opts, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if timeoutSeconds < 1 {
		return opts, errors.New("--timeout must be at least 1 second")
	}
	opts.timeout = time.Duration(timeoutSeconds) * time.Second
	return opts, nil
}

func codexHome() (string, error) {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return expandPath(home)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

func loadProvider() (providerSelection, error) {
	home, err := codexHome()
	if err != nil {
		return providerSelection{}, err
	}
	configPath := filepath.Join(home, "config.toml")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return providerSelection{}, fmt.Errorf("Codex config not found: %s", configPath)
	}
	var config codexConfig
	if err := toml.Unmarshal(raw, &config); err != nil {
		return providerSelection{}, fmt.Errorf("parse Codex config %s: %w", configPath, err)
	}
	providerID := config.ModelProvider
	if providerID == "" {
		providerID = "openai"
	}
	provider, ok := config.ModelProviders[providerID]
	if !ok {
		if providerID == "openai" {
			baseURL := config.OpenAIBaseURL
			if baseURL == "" {
				baseURL = "https://api.openai.com/v1"
			}
			provider = providerConfig{Name: "openai", BaseURL: baseURL, RequiresOpenAIAuth: true}
		} else {
			return providerSelection{}, fmt.Errorf("provider %q is not defined in %s", providerID, configPath)
		}
	}
	if strings.TrimSpace(provider.BaseURL) == "" {
		return providerSelection{}, fmt.Errorf("provider %q has no base_url", providerID)
	}
	return providerSelection{
		ID: providerID, Provider: provider, Home: home,
		Official: isOfficialOpenAIURL(provider.BaseURL),
	}, nil
}

func isOfficialOpenAIURL(baseURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "api.openai.com")
}

func loadAuthMode(home string) string {
	raw, err := os.ReadFile(filepath.Join(home, "auth.json"))
	if err != nil {
		return "unknown"
	}
	var auth struct {
		Mode string `json:"auth_mode"`
	}
	if json.Unmarshal(raw, &auth) != nil || strings.TrimSpace(auth.Mode) == "" {
		return "unknown"
	}
	return strings.TrimSpace(auth.Mode)
}

func resolveToken(provider providerConfig, home string) (string, string, error) {
	if provider.EnvKey != "" {
		if token := strings.TrimSpace(os.Getenv(provider.EnvKey)); token != "" {
			return token, "environment variable " + provider.EnvKey, nil
		}
	}
	if provider.Auth != nil && provider.Auth.Command != "" {
		timeout := provider.Auth.TimeoutMS
		if timeout < 1 {
			timeout = 5000
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
		defer cancel()
		command := exec.CommandContext(ctx, provider.Auth.Command, provider.Auth.Args...)
		if provider.Auth.CWD != "" {
			cwd, err := expandPath(provider.Auth.CWD)
			if err != nil {
				return "", "", fmt.Errorf("resolve provider auth cwd: %w", err)
			}
			command.Dir = cwd
		}
		var output bytes.Buffer
		command.Stdout = &output
		if err := command.Run(); err != nil {
			if ctx.Err() != nil {
				return "", "", errors.New("provider auth command timed out")
			}
			return "", "", fmt.Errorf("provider auth command failed: %w", err)
		}
		if token := strings.TrimSpace(output.String()); token != "" {
			return token, "provider auth command", nil
		}
	}
	if token := strings.TrimSpace(provider.ExperimentalBearerToken); token != "" {
		return token, "provider bearer token", nil
	}
	if provider.RequiresOpenAIAuth {
		raw, err := os.ReadFile(filepath.Join(home, "auth.json"))
		if err == nil {
			var auth struct {
				APIKey string `json:"OPENAI_API_KEY"`
			}
			if json.Unmarshal(raw, &auth) == nil && strings.TrimSpace(auth.APIKey) != "" {
				return strings.TrimSpace(auth.APIKey), "Codex auth.json API key", nil
			}
		}
	}
	return "", "", errors.New("no provider token found; configure env_key, provider auth, or Codex API-key login")
}

func expandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func requestJSON(client *http.Client, method, endpoint, token string, payload, destination any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", userAgent)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return performRequest(client, request, token, destination)
}

type uploadFile struct {
	Field string
	Path  string
}

func requestMultipart(client *http.Client, endpoint, token string, fields map[string]string, files []uploadFile, destination any) error {
	for index := range files {
		path, err := expandPath(files[index].Path)
		if err != nil {
			return err
		}
		files[index].Path = path
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("input image not found: %s", path)
		}
		if info.Size() == 0 {
			return fmt.Errorf("input image is empty: %s", path)
		}
		if info.Size() > maxEditSize {
			return fmt.Errorf("input image exceeds 50 MB: %s", path)
		}
	}

	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	go func() {
		var writeErr error
		defer func() { _ = writer.CloseWithError(writeErr) }()
		keys := make([]string, 0, len(fields))
		for key := range fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := multipartWriter.WriteField(key, fields[key]); err != nil {
				writeErr = err
				return
			}
		}
		for _, file := range files {
			if err := writeMultipartFile(multipartWriter, file); err != nil {
				writeErr = err
				return
			}
		}
		writeErr = multipartWriter.Close()
	}()

	request, err := http.NewRequest(http.MethodPost, endpoint, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	request.Header.Set("User-Agent", userAgent)
	return performRequest(client, request, token, destination)
}

func writeMultipartFile(writer *multipart.Writer, file uploadFile) error {
	source, err := os.Open(file.Path)
	if err != nil {
		return err
	}
	defer source.Close()
	name := strings.NewReplacer(`"`, "_", "\r", "_", "\n", "_").Replace(filepath.Base(file.Path))
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, file.Field, name))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, source)
	return err
}

func performRequest(client *http.Client, request *http.Request, token string, destination any) error {
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("request failed for %s: %s", request.URL, redact(err.Error(), token))
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 4000))
		return fmt.Errorf("HTTP %d from %s: %s", response.StatusCode, request.URL, redact(string(detail), token))
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64*1024*1024))
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("non-JSON response from %s: %w", request.URL, err)
	}
	return nil
}

func likelyImageModels(response map[string]any) ([]string, *int) {
	raw, ok := response["data"].([]any)
	if !ok {
		return []string{}, nil
	}
	markers := []string{"image", "dall", "flux", "imagen", "seedream", "cogview", "kolors"}
	seen := make(map[string]bool)
	for _, value := range raw {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		model, ok := item["id"].(string)
		if !ok {
			continue
		}
		lower := strings.ToLower(model)
		for _, marker := range markers {
			if strings.Contains(lower, marker) {
				seen[model] = true
				break
			}
		}
	}
	models := make([]string, 0, len(seen))
	for model := range seen {
		models = append(models, model)
	}
	sort.Strings(models)
	total := len(raw)
	return models, &total
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func saveImage(client *http.Client, response imageResponse, outputDir, prefix string) (string, error) {
	if len(response.Data) == 0 {
		return "", errors.New("image response has no data[0] object")
	}
	item := response.Data[0]
	var imageBytes []byte
	var err error
	if item.Base64 != "" {
		imageBytes, err = base64.StdEncoding.Strict().DecodeString(item.Base64)
		if err != nil {
			return "", errors.New("image response contains invalid base64 data")
		}
	} else if item.URL != "" {
		request, requestErr := http.NewRequest(http.MethodGet, item.URL, nil)
		if requestErr != nil {
			return "", requestErr
		}
		result, requestErr := client.Do(request)
		if requestErr != nil {
			return "", fmt.Errorf("failed to download generated image: %w", requestErr)
		}
		defer result.Body.Close()
		if result.StatusCode < 200 || result.StatusCode >= 300 {
			return "", fmt.Errorf("failed to download generated image: HTTP %d", result.StatusCode)
		}
		imageBytes, err = io.ReadAll(io.LimitReader(result.Body, 100*1024*1024))
		if err != nil {
			return "", fmt.Errorf("failed to download generated image: %w", err)
		}
	} else {
		return "", errors.New("image response has neither b64_json nor url")
	}

	directory, err := expandPath(outputDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	random, err := randomHex(3)
	if err != nil {
		return "", err
	}
	filename := fmt.Sprintf("%s-%s-%s%s", prefix, time.Now().Format("20060102-150405"), random, imageExtension(item, response.OutputFormat))
	path := filepath.Join(directory, filename)
	if err := os.WriteFile(path, imageBytes, 0o644); err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

func imageExtension(item imageItem, outputFormat string) string {
	if outputFormat != "" {
		format := strings.ToLower(strings.TrimPrefix(outputFormat, "."))
		if format == "jpeg" {
			format = "jpg"
		}
		return "." + format
	}
	if item.URL != "" {
		if parsed, err := url.Parse(item.URL); err == nil {
			extension := strings.ToLower(filepath.Ext(parsed.Path))
			if extension == ".jpeg" {
				return ".jpg"
			}
			if extension == ".png" || extension == ".jpg" || extension == ".webp" {
				return extension
			}
		}
	}
	return ".png"
}

func redact(text, token string) string {
	if token == "" {
		return text
	}
	return strings.ReplaceAll(text, token, "<redacted>")
}

func randomHex(bytesCount int) (string, error) {
	raw := make([]byte, bytesCount)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func writeJSON(writer io.Writer, value outputResult) {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}
