# Generation Image for API — Codex 安装文档

这是一个跨平台 Codex Skill，用于通过 NewAPI 或其他 OpenAI 兼容第三方 API 生成、编辑和合成图片。它会自动判断当前 Provider：第三方 API 会先检查图片模型；OpenAI 官方 Provider 会提示 Codex 改用官方内置生图能力。

## 一句话安装

将下面这一句话发送给 Codex App：

```text
请联网读取并执行这个安装文档：https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/README.md
```

## 给 Codex App 的安装指令

如果你是因为用户发送了上面的提示词而读取本文档，请直接完成安装，不要让用户手工下载文件。

1. 确定 Codex Home：优先使用 `CODEX_HOME`，否则使用用户目录下的 `.codex`。
2. 安装目录为 `<CODEX_HOME>/skills/generation-image-for-api`。如果该目录已存在，停止并告诉用户，不要自行覆盖或删除。
3. 判断当前操作系统和 CPU 架构，只下载公共文件和当前平台对应的一个二进制文件。
4. 所有资源必须从下面列出的 `raw.githubusercontent.com` 地址下载，不需要克隆或访问 GitHub 仓库页面。
5. macOS/Linux 下载后为二进制添加可执行权限；Windows 不需要。
6. 对照 `checksums.sha256` 中当前平台对应的记录验证二进制 SHA-256。
7. 运行二进制的 `--check`。不得输出、复制或记录 API Key。
8. 安装成功后告诉用户：该 Skill 将在下一轮对话中可用。

## 公共文件

保持下列相对路径下载：

| 安装路径 | RAW 下载地址 |
|---|---|
| `SKILL.md` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/SKILL.md` |
| `README.md` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/README.md` |
| `agents/openai.yaml` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/agents/openai.yaml` |
| `checksums.sha256` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/checksums.sha256` |

## 平台二进制

只下载与当前平台匹配的一项，并保持表中的安装路径：

| 平台 | 安装路径 | RAW 下载地址 |
|---|---|---|
| macOS Apple Silicon | `scripts/bin/darwin-arm64/image-api` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/darwin-arm64/image-api` |
| macOS Intel | `scripts/bin/darwin-amd64/image-api` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/darwin-amd64/image-api` |
| Linux ARM64 | `scripts/bin/linux-arm64/image-api` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/linux-arm64/image-api` |
| Linux x86-64 | `scripts/bin/linux-amd64/image-api` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/linux-amd64/image-api` |
| Windows ARM64 | `scripts/bin/windows-arm64/image-api.exe` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/windows-arm64/image-api.exe` |
| Windows x86-64 | `scripts/bin/windows-amd64/image-api.exe` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/windows-amd64/image-api.exe` |

## 权限与验证

macOS/Linux：

```bash
chmod +x "<CODEX_HOME>/skills/generation-image-for-api/scripts/bin/<platform>/image-api"
```

从安装目录计算当前二进制 SHA-256，并与 `checksums.sha256` 中相同相对路径的记录比较。验证一致后执行：

```bash
"<CODEX_HOME>/skills/generation-image-for-api/scripts/bin/<platform>/image-api" --check
```

Windows 执行对应的 `image-api.exe --check`。

正常结果会包含：

- `third_party`：当前是否为第三方 API。
- `image_supported`：第三方 API 是否暴露图片模型。
- `image_models`：识别到的图片模型。
- `use_codex_imagegen`：是否应改用 Codex 官方生图。

## 使用示例

```text
使用 $generation-image-for-api 生成一张赛博朋克城市夜景。
```

```text
使用 $generation-image-for-api，把这张图片的背景改成雨夜霓虹城市，保留人物不变。
```

运行时不需要 Python 或 Go。API Key 仅从 Codex 配置、环境变量或认证命令中动态读取，不会写入下载资源或图片文件。
