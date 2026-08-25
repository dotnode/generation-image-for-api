---
name: generation-image-for-api
description: Generate or edit raster images through NewAPI or another third-party OpenAI-compatible provider. Use for ordinary requests to create photos, illustrations, mockups, textures, sprites, reference-guided variants, background or object changes, compositing, masks, multi-image edits, and other bitmap assets. Route official OpenAI providers to Codex built-in image generation. Do not use for SVG, vector, HTML/CSS, canvas, or other code-native visuals that are better edited directly.
---

# Third-Party Image Generation and Editing

Create and edit bitmap images through the current Codex provider. On a supported third-party API, use the bundled native Go CLI. On an official OpenAI provider, use Codex's built-in image generation capability instead. The third-party path requires neither Python nor a Go installation at runtime.

Treat this as the normal image Skill for a supported third-party provider, not merely as a Provider diagnostic. A successful request is not complete until the image has been inspected and displayed.

## Routing

Run the matching bundled binary with `--check` before an ordinary image request:

```bash
<image-api> --check
```

Interpret the sanitized JSON result:

- `third_party: true` and `image_supported: true`: use this CLI. Prefer a model listed in `image_models`.
- `third_party: false` and `use_codex_imagegen: true`: use Codex's built-in image generation tool for the current request.
- `third_party: true` and `image_supported: false`: do not make a billable request; report that the provider exposes no recognizable image model.
- Verification error: report the sanitized error. Do not guess that the route works and do not expose credentials.

Provider classification uses the effective API hostname. A non-OpenAI hostname remains third-party even when it reuses an OpenAI-compatible credential field.

Use `--sync-routing` only during installation, after a Provider/login change, or when repairing image routing. It enables this Skill and disables the system `imagegen` Skill for a supported third-party image API; it performs the reverse for an official OpenAI route. Restart Codex after routing changes.

## When to use

- Create a new photo, illustration, product image, cover, banner, texture, sprite, mockup, concept, or other raster asset.
- Create a new image using references for subject, identity, style, composition, palette, or mood.
- Edit an image: inpainting, recoloring, relighting, weather changes, background replacement, object removal or insertion, text replacement, cutouts, and style transfer.
- Combine multiple images while preserving selected subjects or scene elements.
- Apply a user-supplied mask to the first edit image.

## When not to use

- The requested result should remain SVG, vector, HTML/CSS, canvas, or another deterministic code-native format.
- An existing project asset is already editable in its native vector or code format and only needs a small deterministic change.
- A simple diagram, shape, or interface component is more reliably built directly in code.

## Decide the image intent

Choose the semantic intent before calling the API:

- **Generate:** no input image is needed.
- **Reference-guided creation:** input images guide subject, identity, style, composition, or mood, but the user is asking for a new image. The CLI still sends these files through the edits endpoint because that is the compatible reference-image transport.
- **Edit:** the user wants to change an existing image while preserving specified parts.
- **Composite:** several inputs contribute different elements to one result.

For every input, assign a numbered role in the prompt:

```text
Image 1: edit target and base composition
Image 2: subject to insert
Image 3: lighting and color reference only
```

Do not assume every attached image is an edit target. For multiple distinct deliverable assets, make separate calls with separate prompts. A variant of one concept and a set of different assets are not the same task.

## Platform binary

Select exactly one executable:

| Platform | Skill-relative executable |
|---|---|
| macOS Apple Silicon | `scripts/bin/darwin-arm64/image-api` |
| macOS Intel | `scripts/bin/darwin-amd64/image-api` |
| Linux ARM64 | `scripts/bin/linux-arm64/image-api` |
| Linux x86-64 | `scripts/bin/linux-amd64/image-api` |
| Windows ARM64 | `scripts/bin/windows-arm64/image-api.exe` |
| Windows x86-64 | `scripts/bin/windows-amd64/image-api.exe` |

Resolve the Skill root from `${CODEX_HOME:-$HOME/.codex}/skills/generation-image-for-api` on macOS/Linux. On Windows, use `%CODEX_HOME%\skills\generation-image-for-api` when `CODEX_HOME` is set, otherwise `%USERPROFILE%\.codex\skills\generation-image-for-api`.

On macOS/Linux, ensure the binary is executable. Use `<image-api> --version` to verify that the executable matches the root `VERSION` file during installation or update.

## Image workflow

1. Run `--check` and select the route.
2. Decide generate, reference-guided creation, edit, or composite.
3. Decide whether the result is preview-only or a project deliverable.
4. Collect the prompt, exact text, input images, image roles, invariants, exclusions, intended size, and quality.
5. Inspect every edit target or important reference with `view_image` before the request.
6. Shape the prompt using [references/prompting.md](references/prompting.md). For complex edits, compositing, localization, identity preservation, or transparency, also read [references/editing.md](references/editing.md).
7. Use one image request unless the user asks for multiple outputs. Do not generate unrequested variants.
8. Inspect the returned file with `view_image`. Validate the subject, composition, style, exact text, edit boundaries, and all invariants.
9. If correction is needed, make one targeted change and repeat the invariants instead of rewriting the whole request.
10. Display the final image with an absolute Markdown image path. For a project deliverable, place the selected output inside the workspace and report its final path.

## Generate

Use `medium` quality for ordinary final work. Use `low` only for connectivity tests, thumbnails, or an explicitly requested fast draft.

```bash
<image-api> \
  --prompt "A polished product photograph of a matte ceramic mug, soft studio lighting, no logo or watermark" \
  --model gpt-image-2 \
  --size 1024x1024 \
  --quality medium
```

## Edit, references, and compositing

Repeat `--edit` in the same order used by the numbered image roles. Use `--mask` only when a mask is actually supplied; it applies to Image 1.

```bash
<image-api> \
  --edit "/absolute/path/base.png" \
  --edit "/absolute/path/subject.png" \
  --prompt "Image 1 is the base scene. Insert the subject from Image 2. Match scale, perspective, lighting, color temperature, contact shadows, and edge sharpness. Keep Image 1 framing and all unrelated objects unchanged." \
  --model gpt-image-2 \
  --quality medium
```

Before any edit, state what changes and what must remain unchanged. Save non-destructively and never overwrite an input image unless the user explicitly requests replacement.

## Output handling

- The CLI defaults to `output/generation-image-for-api` relative to the current working directory and creates a unique filename.
- If the user specifies a destination directory, pass `--output-dir`.
- If the user specifies an exact final filename, generate first and then copy or move the selected result there without overwriting an existing file unless replacement was explicit.
- Preview-only outputs may remain in the generated output directory.
- Project assets must be placed in the workspace before finishing; do not leave a project reference pointing only to an external Codex or temporary directory.
- Always report the final path, operation, model, size, quality, and material prompt constraints.

## Provider and credential safety

The CLI reads the active provider and `base_url` from Codex `config.toml`. It resolves a third-party bearer token from the provider's configured environment variable, command-backed auth, experimental bearer token, or compatible Codex authentication storage.

- Never print, paste, log, or persist the resolved token.
- Never copy credentials into this Skill, prompts, generated images, or output metadata.
- A direct image request authorizes that requested generation or edit, but not extra variants or repeated billable retries.
- Retry a billable request at most once, and only for a plausibly transient failure. Do not retry invalid parameters, missing models, safety errors, or unsupported capabilities.
- The provider must expose compatible `/images/generations` and `/images/edits` endpoints returning `data[0].b64_json` or `data[0].url`.
- The current CLI accepts up to five input images. A single mask applies to the first image.
- If a requested control is unsupported by the selected model or provider, report the exact sanitized error rather than silently changing the model or dropping the requirement.

## Installation and routing maintenance

Use:

```bash
<image-api> --sync-routing
```

The command checks the Provider before changing configuration, migrates legacy directory-only Skill entries to exact `SKILL.md` paths, deduplicates matching entries, writes atomically, and creates a timestamped `config.toml` backup. A successful third-party route must end with this Skill enabled and the system `imagegen` Skill disabled. Fully restart Codex after any routing change.

## Development

The source is in `cmd/image-api`. Keep the source `appVersion` and root `VERSION` identical. Rebuild with Go 1.23 or newer:

```bash
CGO_ENABLED=0 GOOS=<darwin|linux|windows> GOARCH=<arm64|amd64> \
  go build -trimpath -ldflags="-s -w" -o <output-path> ./cmd/image-api
```
