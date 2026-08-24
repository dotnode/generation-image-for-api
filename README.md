# Generation Image for API

这是一个跨平台 Codex Skill，用于通过 Codex 当前配置的 NewAPI 或其他 OpenAI 兼容第三方 API 生成、编辑和合成图片。

它会先检查当前 Provider：

- 第三方 API：查询 `/models`，确认存在图片模型后再调用 `/images/generations` 或 `/images/edits`。
- OpenAI 官方 Provider：不调用本程序，提示 Codex 使用官方内置 `imagegen` 能力。

运行时不需要安装 Python 或 Go。仓库包含 macOS、Linux 和 Windows 的 amd64/arm64 二进制程序。

## Codex App 一句话安装

将下面这句话直接发送给 Codex App：

```text
请用 $skill-installer 安装 dotnode/generation-image-for-api，并按仓库 README 完成权限设置和安装验证。
```

## Codex 安装要求

Codex 读取本 README 后，应按以下参数调用系统 `skill-installer`：

```text
repo: dotnode/generation-image-for-api
ref: main
path: .
name: generation-image-for-api
method: download
```

macOS/Linux 下的等价安装命令：

```bash
python3 "${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-installer/scripts/install-skill-from-github.py" \
  --repo dotnode/generation-image-for-api \
  --ref main \
  --path . \
  --name generation-image-for-api \
  --method download
```

安装器遇到已经存在的 `generation-image-for-api` 目录时应停止，不要自行覆盖或删除。

### macOS 和 Linux 权限

GitHub 下载的 ZIP 可能丢失可执行权限。安装后只需为当前平台对应的二进制添加权限：

```bash
chmod +x "${CODEX_HOME:-$HOME/.codex}/skills/generation-image-for-api/scripts/bin/<platform>/image-api"
```

`<platform>` 可选值：

- Apple Silicon Mac：`darwin-arm64`
- Intel Mac：`darwin-amd64`
- ARM64 Linux：`linux-arm64`
- x86-64 Linux：`linux-amd64`

Windows 不需要执行 `chmod`，选择 `windows-arm64/image-api.exe` 或 `windows-amd64/image-api.exe`。

### 安装验证

运行当前平台程序的非计费检查：

```bash
<image-api> --check
```

验证成功后，Codex 应报告 Provider 类型、图片模型支持情况，并提醒该 Skill 将在下一轮对话中可用。不得输出或记录 API Key。

## 使用示例

安装后的 Skill 名称是 `$generation-image-for-api`：

```text
使用 $generation-image-for-api 生成一张赛博朋克城市夜景。
```

编辑图片时可以提供一张或多张本地图片，也可以附带蒙版：

```text
使用 $generation-image-for-api，把这张图片的背景改成雨夜霓虹城市，保留人物不变。
```

## 源码与安全

- Go 源码：`cmd/image-api/main.go`
- 单元测试：`cmd/image-api/main_test.go`
- API Key 仅从 Codex 配置、环境变量或认证命令中动态读取。
- Skill 不会把 API Key 写入仓库、图片文件或命令输出。
