# buildmux

把单次 `buildctl build` 调用拆分到多台按平台划分的 buildkitd 后端并发执行，再用 `manifest-tool` 合并成一个 manifest list。

适用场景：在调度机上同时驱动一台 amd64 buildkitd 和一台 arm64 buildkitd（或更多），跳过本机 QEMU 模拟，获得接近原生的多架构构建速度。

## 工作原理

```
buildmux build [buildctl 参数]
   │
   ├── 解析 --opt platform=... 和 --output
   ├── 加载 YAML 配置（platform → endpoint + tag 模板）
   │
   ├── 为每个 platform 并发拉起 buildctl 子进程
   │   - --addr 指向该平台对应的 buildkitd
   │   - --opt platform 替换为单值
   │   - --output name 按 tag 模板渲染（如 foo:v1-amd64）
   │
   │  任一失败 → ctx.Cancel() → 其他子进程被 SIGTERM，不进入下一步
   │
   └── 所有子进程成功 → manifest-tool push from-spec 合并
```

详细设计见 [`docs/superpowers/specs/2026-05-14-buildmux-design.md`](docs/superpowers/specs/2026-05-14-buildmux-design.md)。

## 前置依赖

运行时：
- `buildctl`（在 PATH 中，或用 `--buildctl` 指定）
- `manifest-tool`（同上，或用 `--manifest-tool` 指定）
- 一组可访问的 buildkitd endpoint

构建时：
- Go 1.22+

## 构建与安装

```bash
git clone https://github.com/chwetion/buildmux && cd buildmux
go build -o buildmux ./cmd/buildmux
```

可选：放到 PATH

```bash
install -m 0755 buildmux /usr/local/bin/
```

## 配置文件

默认从 `./buildmux.yaml` 读取；可通过 `--config` 或 `$BUILDMUX_CONFIG` 覆盖。

```yaml
version: 1

platforms:
  # key 必须与 buildctl --opt platform=... 取值完全一致
  linux/amd64:
    endpoint: tcp://buildkit-amd64.internal:1234
    tag: "{{.Name}}-amd64"          # tag 模板（Go text/template）
    tls:                            # 整块可选；没 tls 块即明文连接
      ca_cert: /etc/buildmux/ca.pem
      cert:    /etc/buildmux/client.pem
      key:     /etc/buildmux/client.key
      server_name: buildkit-amd64.internal   # 可选

  linux/arm64:
    endpoint: unix:///run/buildkit/buildkitd.sock
    tag: "{{.Name}}-arm64"

  linux/arm/v7:
    endpoint: tcp://buildkit-armv7.internal:1234
    tag: "{{.Repo}}:{{.Tag}}-armv7"

manifest:
  insecure: false                   # 推送到 HTTP registry 时设 true
  username: ""                      # 留空则走 ~/.docker/config.json
  password: ""
```

### tag 模板可用变量

| 变量 | 含义 | 示例（对 `linux/arm/v7` + `docker.io/me/app:v1`） |
|---|---|---|
| `{{.Name}}` | 完整原始 name（含 tag） | `docker.io/me/app:v1` |
| `{{.Repo}}` | name 去掉 `:tag` 部分 | `docker.io/me/app` |
| `{{.Tag}}`  | tag 部分（无 tag 时为 `latest`） | `v1` |
| `{{.OS}}` | 平台 OS 段 | `linux` |
| `{{.Arch}}` | 平台 arch 段 | `arm` |
| `{{.Variant}}` | 平台 variant 段（可能为空） | `v7` |

`examples/buildmux.yaml` 是一份可直接修改的示例。

## 使用

CLI 完全兼容 `buildctl build`。把 `buildctl` 换成 `buildmux`，其余参数原样照传：

```bash
buildmux build \
  --config buildmux.yaml \
  --frontend dockerfile.v0 \
  --local context=. \
  --local dockerfile=. \
  --opt platform=linux/amd64,linux/arm64 \
  --output type=image,name=docker.io/me/app:v1,push=true
```

输出会按平台前缀分流，stdout 是 TTY 时自动着色：

```
[linux/amd64] #1 [internal] load build definition from Dockerfile
[linux/arm64] #1 [internal] load build definition from Dockerfile
[linux/amd64] #5 exporting to image
[manifest-tool] Digest: sha256:...
```

### 其他子命令

```bash
buildmux version    # 打印版本、commit、构建日期；也支持 --version / -v
buildmux help       # 打印用法；也支持 --help / -h
```

### buildmux 自己的 flag

| flag | 说明 | 默认 |
|---|---|---|
| `--config <path>` | YAML 配置文件 | `./buildmux.yaml` 或 `$BUILDMUX_CONFIG` |
| `--buildctl <path>` | buildctl 二进制路径 | PATH 查找 |
| `--manifest-tool <path>` | manifest-tool 二进制路径 | PATH 查找 |

其余参数全部原样转发给 `buildctl`，除了：

- `--opt platform=...`：buildmux 拦截、拆分后按平台分发
- `--output`：buildmux 解析后按 tag 模板重写 `name=`

如果 `buildctl` 的某个参数名与 buildmux 自有 flag 冲突，可用 `--` 分隔：

```bash
buildmux build --config buildmux.yaml -- --frontend dockerfile.v0 ...
```

### 限制

- `--output` 必须是 `type=image,push=true,name=...`，其他形式（oci / local / tar）报错退出。
- `--opt platform=...` 是必填，未指定时报错（不会回退到本机架构）。
- 所有列在 `--opt platform=` 的平台都必须在 YAML 中有对应条目，否则报错并列出缺失项。

### 退出码

| 退出码 | 含义 |
|---|---|
| 0 | 全部成功 |
| 2 | 配置错误 / 参数错误 / 校验失败 |
| 127 | buildctl 或 manifest-tool 二进制找不到 |
| 130 | 收到 SIGINT/SIGTERM 被中断 |
| 其他 | 透传自 buildctl 或 manifest-tool 的退出码（任一失败时） |

## 开发

### 项目结构

```
cmd/buildmux/main.go          CLI 入口（flag 解析、信号处理、退出码）
internal/cli/args/            buildctl args 解析（纯函数）
internal/cli/build/           build 子命令编排（load config → dispatch → manifest）
internal/config/              YAML schema、Load/Validate、tag 模板渲染
internal/output/              --output KV 字符串解析/序列化
internal/dispatch/            errgroup 包装、fail-fast 并发调度
internal/exec/                buildctl/manifest-tool 子进程封装、PrefixWriter
testdata/stubs/               集成测试用的 fake-buildctl / fake-manifest-tool
examples/buildmux.yaml        示例配置
docs/superpowers/specs/       设计文档
docs/superpowers/plans/       实现计划（task 拆分）
```

### 跑测试

```bash
# 单元测试
go test ./...

# 单元 + 集成测试（用桩二进制，不依赖真 buildctl / manifest-tool / registry）
go test -tags=integration ./...

# 静态检查
go vet ./...
```

### 端到端 smoke（无外网、无 buildkitd）

桩二进制只打印自己收到的参数，方便人工核对 buildmux 重写后的命令行：

```bash
mkdir -p testdata/bin
go build -o testdata/bin/fake-buildctl ./testdata/stubs/fake-buildctl
go build -o testdata/bin/fake-manifest-tool ./testdata/stubs/fake-manifest-tool

./buildmux build \
  --config examples/buildmux.yaml \
  --buildctl ./testdata/bin/fake-buildctl \
  --manifest-tool ./testdata/bin/fake-manifest-tool \
  --frontend dockerfile.v0 --local context=. \
  --opt platform=linux/amd64,linux/arm64 \
  --output type=image,name=me/app:v1,push=true
```

预期看到三行 `[linux/amd64]` / `[linux/arm64]` / `[manifest-tool]` 输出，退出码 0。

### 新增平台

只要在 `buildmux.yaml` 的 `platforms:` 下加一个 key（如 `linux/ppc64le`），指定它的 `endpoint` 和 `tag`，就可以在调用时把它写进 `--opt platform=`。无需改代码。

## 发布

发布流程基于 [GoReleaser](https://goreleaser.com) + GitHub Actions（配置见 `.goreleaser.yaml` 和 `.github/workflows/release.yml`）。

打 tag 即触发：

```bash
git tag v0.1.0
git push origin v0.1.0
```

GH Actions 会：

1. 跑一次 `go test ./...` 和 `go vet ./...`
2. 用 GoReleaser 交叉编译 `linux/{amd64,arm64}` 和 `darwin/{amd64,arm64}` 共 4 份二进制
3. 通过 `-ldflags` 注入版本号、commit 和构建日期（`buildmux version` 可看到）
4. 打成 `buildmux_<version>_<os>_<arch>.tar.gz`（含二进制 + README + 示例配置）
5. 生成 `checksums.txt`（SHA256）
6. 创建/更新对应的 GitHub Release，附上由 conventional commits 生成的 changelog

本地干跑（不推送 tag）：

```bash
goreleaser release --snapshot --clean
```

产物会出现在 `dist/` 目录。

## 许可

MIT
