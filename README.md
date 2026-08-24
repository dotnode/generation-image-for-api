# Generation Image for API — Codex 安装与更新文档

这是一个跨平台 Codex Skill，用于通过 NewAPI 或其他 OpenAI 兼容第三方 API 生成、编辑和合成图片。它会自动判断当前 Provider：第三方 API 会先检查图片模型；OpenAI 官方 Provider 会启用 Codex 内置的官方图片 Skill。

本文档同时适用于：

- 首次安装。
- 覆盖更新到 `main` 分支的最新版本。
- 更换 API Provider 或登录方式后重新同步图片 Skill 路由。

## 一句话安装或更新

无论电脑上是否已经安装，都将下面这一句话发送给 Codex App：

```text
请联网读取并执行这个安装文档：https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/README.md
```

同一句提示词可以反复使用：未安装时执行首次安装，已经安装时执行安全更新。不要把 GitHub 仓库地址、压缩包地址或旧版 README 地址补充到提示词中。

## Codex App 必须执行的流程

如果你是因为用户发送了上面的提示词而读取本文档，请直接完成整个流程，不要只总结本文档，也不要要求用户手工下载文件。

### 1. 确定安装位置和运行平台

1. Codex Home 优先使用环境变量 `CODEX_HOME`；未设置时使用当前用户目录下的 `.codex`。
2. 正式安装目录固定为 `<CODEX_HOME>/skills/generation-image-for-api`。
3. 根据操作系统和 CPU 架构，在下方平台表中选择且只选择一个二进制文件。
4. 只允许从本文列出的 `raw.githubusercontent.com` 地址下载资源，不需要克隆仓库，也不需要打开 GitHub 仓库页面。

### 2. 判断首次安装还是更新

- 正式安装目录不存在：这是首次安装。
- 正式安装目录已经存在：这是更新。不得因为目录存在而停止，也不得直接在原目录中逐个覆盖文件。

安装和更新都必须先在同级目录创建临时目录，例如：

```text
<CODEX_HOME>/skills/.generation-image-for-api-stage-<timestamp>
```

将所有公共文件及当前平台的一个二进制文件下载到临时目录，并保持本文规定的相对路径。

### 3. 在替换前完成验证

1. 确认五个公共文件和当前平台二进制均已成功下载，文件不为空。
2. 从临时目录读取 `VERSION`，得到本次应安装的版本号。
3. 从临时目录读取 `checksums.sha256`，计算下载后二进制文件的 SHA-256，并与清单中相同相对路径的记录精确比较。
4. SHA-256 不一致时立即停止，删除临时目录并保留现有安装，不得执行或安装该二进制。
5. macOS/Linux 校验成功后为二进制添加可执行权限；Windows 不需要此步骤。
6. 从临时目录运行当前平台二进制的 `--version`。确认 JSON 中 `ok: true`，并且 `version` 与 `VERSION` 文件完全一致；不一致时停止更新。

### 4. 安装或安全更新

首次安装时，将已验证的临时目录移动为正式安装目录。

更新时：

1. 在 Skill 扫描目录之外创建仅用于目录切换的临时位置，例如 `<CODEX_HOME>/tmp/generation-image-for-api-old-<timestamp>`。
2. 将现有正式安装目录移动到该临时位置。
3. 将已验证的临时目录移动为正式安装目录。
4. 如果新目录无法就位，立即把原目录恢复为正式安装目录并报告更新失败。
5. 新目录成功就位后立即删除旧目录，不保留旧版本备份。

临时下载和校验完成前，不得改变现有可用安装。不要把旧版 Skill 留在 `<CODEX_HOME>/skills` 中，否则它可能被识别成重复 Skill。

### 5. 每次安装或更新后都要同步路由

先从正式安装目录再次运行当前平台二进制的 `--version`，确认它仍与正式目录中的 `VERSION` 一致。这一步用于防止 Windows 更新后继续调用旧的 `image-api.exe`。

版本确认后，必须运行新安装版本中当前平台二进制的 `--sync-routing`。即使之前运行过，更新后也必须重新运行，因为新版可能修改 Provider 检测、模型识别或路由规则。

macOS/Linux：

```bash
"<CODEX_HOME>/skills/generation-image-for-api/scripts/bin/<platform>/image-api" --sync-routing
```

Windows：

```powershell
& "<CODEX_HOME>\skills\generation-image-for-api\scripts\bin\<platform>\image-api.exe" --sync-routing
```

不得输出、复制或记录 API Key。程序只有在检查成功时才可以修改 `config.toml`；配置修改必须由程序原子完成并创建配置备份。

### 6. 检查结果并完成交付

1. 确认命令返回的 JSON 中 `ok` 为 `true`。
2. 确认并报告 `version`，且它与已下载的 `VERSION` 相同。
3. 记录并报告 `third_party`、`image_supported`、`image_models`、`custom_skill_enabled`、`system_imagegen_enabled` 和 `restart_required`。
4. 如果 `routing_changed` 为 `true`，确认并报告 `backup_path` 指向的配置备份确实存在。
5. 检查 `config.toml` 中两个路由条目的 `path` 必须分别以 `generation-image-for-api/SKILL.md` 和 `.system/imagegen/SKILL.md` 结尾。仅指向 Skill 目录的旧记录无效，程序应自动迁移并去重。
6. 如果 `ok` 不为 `true`，不要猜测安装成功；报告错误，并说明原有 Codex 配置未被修改。
7. 明确告诉用户必须完全退出并重启 Codex App，然后新建一个任务。仅新建任务不保证重新加载修改后的 Skill 配置。
8. 建议用户在重启后的新任务中发送一个普通生图请求，验证是否自动选择了正确的图片 Skill。

## 首次运行检查表

首次安装完成必须同时满足：

- 当前平台二进制已经通过 SHA-256 校验。
- 二进制 `--version` 的结果与 `VERSION` 文件一致。
- macOS/Linux 二进制已有可执行权限。
- `--sync-routing` 返回 `ok: true`。
- 路由发生变化时，配置备份路径真实存在。
- 两个 `[[skills.config]].path` 都精确指向对应的 `SKILL.md` 文件，而不是目录。
- 用户已经被告知完全重启 Codex App，并在重启后新建任务。

仅下载文件不算完成安装。

## 更新说明

Skill 更新后仍需完成以下步骤，不能只替换 `SKILL.md`：

1. 重新下载全部公共文件和当前平台二进制。
2. 重新验证新版二进制 SHA-256。
3. 对照 `VERSION` 验证临时目录和正式安装目录中的二进制版本。
4. 用已验证的新目录替换旧目录，成功后不保留旧版 Skill。
5. 使用新版二进制重新运行 `--sync-routing`。
6. 完全退出并重启 Codex App，再新建任务，让新版 Skill 和新的启用状态生效。

因此，更新时最简单可靠的方法仍是再次发送本文顶部的“一句话安装或更新”。

## 什么时候只需重新同步

如果安装文件没有变化，只是更换了 Codex Provider、第三方 API 地址、API Key 或 ChatGPT 登录方式，不必重新下载 Skill。直接运行当前平台程序的 `--sync-routing`；如果 `routing_changed` 为 `true`，完全退出并重启 Codex App，然后新建任务。

## 公共文件

下载时保持下列相对路径：

| 安装路径 | RAW 下载地址 |
|---|---|
| `SKILL.md` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/SKILL.md` |
| `README.md` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/README.md` |
| `agents/openai.yaml` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/agents/openai.yaml` |
| `VERSION` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/VERSION` |
| `checksums.sha256` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/checksums.sha256` |

## 平台二进制

只下载与当前平台匹配的一项，并保持表中的安装路径：

| 平台 | 平台目录 | 安装路径 | RAW 下载地址 |
|---|---|---|---|
| macOS Apple Silicon | `darwin-arm64` | `scripts/bin/darwin-arm64/image-api` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/darwin-arm64/image-api` |
| macOS Intel | `darwin-amd64` | `scripts/bin/darwin-amd64/image-api` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/darwin-amd64/image-api` |
| Linux ARM64 | `linux-arm64` | `scripts/bin/linux-arm64/image-api` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/linux-arm64/image-api` |
| Linux x86-64 | `linux-amd64` | `scripts/bin/linux-amd64/image-api` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/linux-amd64/image-api` |
| Windows ARM64 | `windows-arm64` | `scripts/bin/windows-arm64/image-api.exe` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/windows-arm64/image-api.exe` |
| Windows x86-64 | `windows-amd64` | `scripts/bin/windows-amd64/image-api.exe` | `https://raw.githubusercontent.com/dotnode/generation-image-for-api/main/scripts/bin/windows-amd64/image-api.exe` |

## 路由结果说明

`--sync-routing` 的主要输出字段：

- `version`：实际运行的 `image-api` 版本，可用于确认 Windows 没有继续调用旧 EXE。
- `third_party`：当前是否为第三方 API。
- `image_supported`：第三方 API 是否暴露图片模型。
- `image_models`：识别到的图片模型。
- `use_codex_imagegen`：是否应改用 Codex 官方生图。
- `custom_skill_enabled`：本 Skill 的最终状态。
- `system_imagegen_enabled`：系统 `imagegen` Skill 的最终状态。
- `routing_changed`：本次是否修改了路由配置。
- `backup_path`：发生配置变更时创建的配置备份文件。
- `restart_required`：路由配置是否发生变化并需要重启 Codex。安装或更新 Skill 文件后，无论该字段为何值，都应重启 Codex。

最终路由规则：

| 检测结果 | 本 Skill | 系统 `imagegen` |
|---|---:|---:|
| 第三方 API 且支持图片模型 | 启用 | 禁用 |
| OpenAI 官方 Provider | 禁用 | 启用 |
| 第三方无图片模型，但存在 ChatGPT 官方登录 | 禁用 | 启用 |
| 检查失败或没有可用图片路线 | 保持原状 | 保持原状 |

## 使用示例

重启 Codex 并新建任务后，可以直接描述图片需求，也可以明确指定 Skill：

```text
使用 $generation-image-for-api 生成一张赛博朋克城市夜景。
```

```text
使用 $generation-image-for-api，把这张图片的背景改成雨夜霓虹城市，保留人物不变。
```

运行时不需要 Python 或 Go。API Key 仅从 Codex 配置、环境变量或认证命令中动态读取，不会写入下载资源或图片文件。
