---
name: generation-image-for-api
description: Detect whether Codex uses an official OpenAI provider or a third-party OpenAI-compatible API, verify third-party image-model support, and generate or edit images through supported custom providers. Route official Codex logins to the built-in image generation capability.
---

# NewAPI Image Generation and Editing

Use the bundled native Go CLI to classify the current Codex provider, synchronize image-Skill enablement, and generate images. For a third-party API, it verifies `/models` and then supports generation through `/images/generations` and editing through multipart `/images/edits`. It requires neither Python nor a Go installation at runtime.

Always run the matching bundled binary from the platform table below before choosing an image route. If it reports an official OpenAI provider, use the built-in `image_gen` tool for the current request; synchronized future tasks use Codex's system `imagegen` Skill. If it reports a supported third-party API, use this CLI and do not call the built-in tool. A successful generation is not complete until the returned image is displayed.

## Platform binary

Select exactly one executable for the current operating system and architecture:

| Platform | Skill-relative executable |
|---|---|
| macOS Apple Silicon | `scripts/bin/darwin-arm64/image-api` |
| macOS Intel | `scripts/bin/darwin-amd64/image-api` |
| Linux ARM64 | `scripts/bin/linux-arm64/image-api` |
| Linux x86-64 | `scripts/bin/linux-amd64/image-api` |
| Windows ARM64 | `scripts/bin/windows-arm64/image-api.exe` |
| Windows x86-64 | `scripts/bin/windows-amd64/image-api.exe` |

Resolve the Skill root from `${CODEX_HOME:-$HOME/.codex}/skills/generation-image-for-api` on macOS/Linux. On Windows, use `%CODEX_HOME%\skills\generation-image-for-api` when `CODEX_HOME` is set, otherwise `%USERPROFILE%\.codex\skills\generation-image-for-api`. In the examples below, replace `<image-api>` with the absolute path to the selected executable.

On macOS or Linux, ensure the selected binary is executable before its first run. GitHub ZIP installation can discard executable permissions:

```bash
chmod +x "<image-api>"
```

## Workflow

1. During installation, after a Provider/login change, or when the user explicitly asks to repair image routing, run:

   ```bash
   <image-api> --sync-routing
   ```

   This command checks the Provider before changing configuration, creates a local backup, and atomically updates the two image Skill entries. Each `[[skills.config]].path` points to the Skill's exact `SKILL.md` file, as required by Codex; legacy directory-only entries are migrated and deduplicated:

   - Supported third-party image API: enable `generation-image-for-api` and disable the system `imagegen` Skill.
   - Official OpenAI Provider: disable `generation-image-for-api` and enable the system `imagegen` Skill.
   - Third-party API without an image model but with an official ChatGPT login: enable the system `imagegen` Skill.
   - Failed verification or no usable image route: leave `config.toml` unchanged.

   After a changed routing configuration, fully restart Codex and then start a new task so the Skill catalog reloads. Installing or updating this Skill also requires a Codex restart even when the route itself was already correct. Do not repeatedly run this mutation during ordinary image requests when the Provider has not changed.

2. For an ordinary request routed to this Skill, run the non-billable model-support check first:

   ```bash
   <image-api> --check
   ```

   Interpret its JSON result before continuing:

   - `third_party: false` and `use_codex_imagegen: true`: stop using the Image API CLI and use the built-in `image_gen` tool for the current request. Run `--sync-routing` only when installation or a Provider change requires future-task routing repair.
   - `third_party: true` and `image_supported: true`: continue with this CLI. Prefer a model listed in `image_models`.
   - `third_party: true` and `image_supported: false`: do not attempt a billable image request. Report that the selected third-party API exposes no recognizable image model.
   - A `/models` verification error: report the exact sanitized error; do not guess that image generation is supported.

   Provider classification is based on the effective API hostname, not `requires_openai_auth`. A custom provider that reuses the Codex API-key field but points to a non-OpenAI hostname remains third-party.

3. To print the same third-party model discovery information explicitly:

   ```bash
   <image-api> --list-models
   ```

4. After the check confirms a third-party image model, generate one image unless the user explicitly requests more. Prefer low quality for a connectivity test:

   ```bash
   <image-api> \
     --prompt "A simple blue circle on a white background" \
     --model gpt-image-2 \
     --quality low
   ```

5. For an edit, pass the existing image with `--edit`. Repeat `--edit` for reference/compositing inputs. Use `--mask` only when the user supplies a mask:

   ```bash
   <image-api> \
     --edit "/absolute/path/source.png" \
     --prompt "Replace only the background with a rainy neon city; preserve the subject" \
     --model gpt-image-2 \
     --quality medium
   ```

   Multiple references:

   ```bash
   <image-api> \
     --edit "/absolute/path/subject.png" \
     --edit "/absolute/path/style-reference.png" \
     --prompt "Keep the subject from image 1 and apply the lighting and palette of image 2"
   ```

6. Before editing, inspect the source image with `view_image` and state the invariants in the prompt. Save non-destructively; never overwrite the source.

7. Read the JSON result from stdout. Inspect the returned local file with `view_image`, then show it with an absolute Markdown image path. Never finish with only a success statement or prompt summary.

## Provider and credential handling

The CLI reads `model_provider` and `model_providers.<id>.base_url` from the platform's Codex `config.toml`. The default OpenAI provider and `https://api.openai.com` are classified as official and routed to Codex's built-in image generation. Other hostnames are classified as third-party. For third-party APIs, the CLI resolves the bearer token from the provider's `env_key`, command-backed `auth`, `experimental_bearer_token`, or—when `requires_openai_auth = true`—the `OPENAI_API_KEY` field in Codex `auth.json`.

- Never print, paste, log, or persist the resolved token.
- Do not copy credentials into this Skill or generated artifacts.
- `--sync-routing` updates only the matching `SKILL.md`-based `[[skills.config]]` entries, removes duplicate legacy directory-path entries, writes through a same-directory temporary file, and creates a timestamped backup before replacement. Do not delete that backup automatically.
- A direct user request to generate an image authorizes the requested generation. Do not create extra variants or retry a billable request more than once unless the user asks.
- The configured service must expose OpenAI-compatible `/images/generations` for generation and `/images/edits` for editing, returning either `data[0].b64_json` or `data[0].url`.
- Editing uploads one to five local images using multipart form data. A mask, when supplied, applies to the first image.
- If the endpoint works but the model is unavailable, report the exact model-routing error and suggest configuring that image model in NewAPI.

## Development

The source is in `cmd/image-api`. Rebuild a platform binary from the Skill root with Go 1.23 or newer:

```bash
CGO_ENABLED=0 GOOS=<darwin|linux|windows> GOARCH=<arm64|amd64> \
  go build -trimpath -ldflags="-s -w" -o <output-path> ./cmd/image-api
```
