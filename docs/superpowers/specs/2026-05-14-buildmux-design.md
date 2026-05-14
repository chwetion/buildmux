# buildmux 设计文档

**日期：** 2026-05-14
**状态：** Draft（待用户最终 review）

## 1. 背景与目标

`buildmux` 是一个 Go 编写的 CLI 工具，把单条 `buildctl build` 调用拆分到一组按平台划分的 buildkitd 后端，并最终通过 `manifest-tool` 把各架构镜像合并成一个 manifest list。

典型场景：在一台调度机上同时驱动一台 amd64 buildkitd 和一台 arm64 buildkitd，避免本机 QEMU 模拟构建的性能损耗，得到原生构建速度的多架构镜像。

非目标：

- 不替代 `buildctl`，不重新实现 BuildKit 协议。
- 不内置镜像签名、SBOM、attestation 等周边能力。
- 不处理 buildkitd 自身的部署与生命周期。

## 2. 总体架构

数据流：

1. 用户调用 `buildmux build [buildctl args...] --config buildmux.yaml`。
2. buildmux 解析 args，抽出 `--opt platform=...` 和 `--output ...`。
3. 加载 YAML，校验每个 platform 都有对应 endpoint 配置。
4. 对每个 platform 重写 args（`--addr`、可选 TLS、单值 `--opt platform`、按模板渲染的 `--output name`），用 `errgroup.WithContext` 并发 `exec.CommandContext` 起 `buildctl` 子进程。
5. 任一子进程失败 → ctx.Cancel()，其余被 SIGTERM/SIGKILL，**不调用** manifest-tool。
6. 所有子进程成功 → 调用 `manifest-tool push from-args` 把各 per-arch tag 合并到用户指定的 target tag。

buildmux 不接触镜像数据流；它只做「args 重写 + 子进程编排」。

## 3. 包结构

```
github.com/chwetion/buildmux/
├── go.mod
├── cmd/buildmux/main.go              入口；解析 buildmux 自有 flag，路由到子命令
├── internal/
│   ├── cli/
│   │   ├── build.go                  "build" 子命令；编排 config→rewrite→dispatch→manifest
│   │   └── args.go                   纯函数：解析/切分/重写 buildctl args
│   ├── config/
│   │   ├── config.go                 YAML schema、Load、Validate
│   │   └── platform.go               Platform 类型、tag 模板渲染
│   ├── dispatch/
│   │   └── dispatch.go               errgroup 并发调度、fail-fast
│   ├── exec/
│   │   ├── buildctl.go               构造并运行 buildctl 命令
│   │   ├── manifest.go               构造并运行 manifest-tool 命令
│   │   └── prefix.go                 带前缀的 io.Writer，转发子进程 stdout/stderr
│   └── output/
│       └── output.go                 解析/校验/重写 --output KV 字符串
├── examples/buildmux.yaml            示例配置
└── testdata/                         单元测试 fixture
```

职责约束：

- `internal/cli/args.go`、`internal/output`、`internal/config` 是纯函数/无 IO，便于表驱动测试。
- `internal/exec` 是唯一调用 `exec.CommandContext` 的地方。
- `cmd/buildmux/main.go` 极薄。

## 4. CLI

用法：

```
buildmux build [buildctl-build-args...] --config buildmux.yaml
```

buildmux 自有 flag（其余原样转发给 buildctl）：

| flag | 说明 | 默认 |
|---|---|---|
| `--config <path>` | YAML 配置文件路径 | `./buildmux.yaml` 或 `$BUILDMUX_CONFIG` |
| `--buildctl <path>` | buildctl 二进制路径 | 在 PATH 中查找 |
| `--manifest-tool <path>` | manifest-tool 二进制路径 | 在 PATH 中查找 |

不引入 `--parallel`、`--fail-fast` —— 默认全并行 + fail-fast，行为不可配。

CLI 框架使用标准库 `flag`，不引入 cobra。

## 5. YAML 配置 schema

```yaml
version: 1

platforms:
  linux/amd64:
    endpoint: tcp://buildkit-amd64.internal:1234
    tag: "{{.Name}}-amd64"
    tls:                                # 整块可选
      ca_cert: /etc/buildmux/ca.pem
      cert:    /etc/buildmux/client.pem
      key:     /etc/buildmux/client.key
      server_name: buildkit-amd64.internal   # 可选

  linux/arm64:
    endpoint: tcp://buildkit-arm64.internal:1234
    tag: "{{.Name}}-arm64"

  linux/arm/v7:
    endpoint: unix:///run/buildkit/buildkitd.sock
    tag: "{{.Name}}-armv7"

manifest:
  insecure: false
  username: ""    # 仅当显式给出才传给 manifest-tool；否则走 ~/.docker/config.json
  password: ""
```

字段规则：

- `platforms` 的 key 取 `os/arch[/variant]` 字符串，与 buildctl `--opt platform=` 取值完全一致。
- `tag` 是 Go `text/template`，渲染上下文：
  - `.Name`：用户 `--output name=foo:v1` 的完整原始 name（含 tag），例如 `docker.io/me/app:v1`
  - `.Repo`：`docker.io/me/app`
  - `.Tag`：`v1`
  - `.OS` / `.Arch` / `.Variant`：平台拆解
- 启动期校验失败的情形：
  - `version != 1`
  - `platforms` 为空
  - 某个 platform 缺 `endpoint` 或 `tag`
  - `tls` 块给了但 `ca_cert`/`cert`/`key` 任一缺失（三件套要么齐要么完全不给）
  - tag 模板解析失败

## 6. 参数解析与重写

`internal/cli/args.go` 是纯函数：

```go
type ParsedArgs struct {
    Platforms []string   // 抽离出来的 --opt platform 值列表
    Output    OutputSpec // 解析后的 --output
    Rest      []string   // 移除 --opt platform 和 --output 后剩下的原始 args
}

func Parse(args []string) (ParsedArgs, error)
```

解析规则：

1. 扫描 args，识别所有 `--opt platform=<v>`、`--opt=platform=<v>`、`--opt platform <v>` 三种形式；值按 `,` 切开累加到 `Platforms`，**对应 token 从 Rest 中剔除**。
2. 找到唯一的 `--output <v>`（支持 `--output=<v>`），调用 `internal/output.Parse`；从 Rest 剔除。
3. 校验：
   - `Platforms` 非空，否则错误：`--opt platform=... is required`
   - `Output.Type == "image"` 且 `Output.Push == true`，否则错误：`only --output type=image,push=true is supported`
   - `Output.Name` 非空
4. 所有 `Platforms` 都必须在 config 的 `platforms` map 中存在；缺失时报错并列出全部缺失项。

对每个 platform `p` 重写后的 buildctl 调用：

```
buildctl \
  --addr <cfg.Platforms[p].Endpoint> \
  [--tlscacert ... --tlscert ... --tlskey ... [--tlsservername ...]]  # 若 tls 存在 \
  build \
  <Rest 原样> \
  --opt platform=<p> \                                 # 单值，覆盖原多值
  --output type=image,name=<渲染后 tag>,push=true,<原 --output 其余 KV 原样保留>
```

只重写 4 项：`--addr`、TLS 三/四件套、`--opt platform`、`--output name`。其余 args 一律原样转发。

## 7. 并发调度

`internal/dispatch` 使用 `errgroup.WithContext(ctx)`：

- 所有 platform 任务同时 Go()。
- 任一任务返回非 nil error → ctx 被取消 → 其他正在运行的 buildctl 子进程被发送 SIGTERM；5 秒未退则 SIGKILL。
- `errgroup.Wait()` 返回首个 error；buildmux 不调用 manifest-tool，直接退出。
- buildmux 自身收到 SIGINT/SIGTERM → 顶层 ctx 取消，所有子进程同样回收。

只有 `errgroup.Wait()` 返回 nil 时，才进入 manifest-tool 合并阶段。

## 8. 子进程输出转发

`internal/exec/prefix.go` 提供一个 io.Writer 包装：

```
[linux/amd64] #1 [internal] load build definition from Dockerfile
[linux/arm64] #1 [internal] load build definition from Dockerfile
[linux/amd64] #5 exporting to image
[manifest-tool] Digest: sha256:...
```

stdout 是 TTY 时，每个 platform 分配一个固定颜色；非 TTY（CI）时纯文本无颜色。

## 9. manifest-tool 调用

所有 per-arch 构建成功后，统一走 `manifest-tool push from-spec`，避免依赖 `--template` 的占位符归一化规则：

```
manifest-tool \
  [--username U --password P] \              # 仅当 config.manifest 给出
  [--insecure] \                             # 仅当 config.manifest.insecure
  push from-spec <临时 spec.yaml>
```

`spec.yaml` 由 buildmux 在内存中生成、写入 `os.CreateTemp` 临时文件，无论成功失败 `defer os.Remove`。格式（manifest-tool 标准 spec 格式）：

```yaml
image: docker.io/me/app:v1                  # 用户 --output name= 的原始 name
manifests:
  - image: docker.io/me/app:v1-amd64        # 按 config.platforms[linux/amd64].tag 渲染
    platform:
      os: linux
      architecture: amd64
  - image: docker.io/me/app:v1-arm64
    platform:
      os: linux
      architecture: arm64
  - image: docker.io/me/app:v1-armv7
    platform:
      os: linux
      architecture: arm
      variant: v7
```

只用 `from-spec` 一条路径：实现简单，不引入对 tag 模板形状的额外约束。

## 10. 错误处理与退出码

| 错误类型 | 时机 | 行为 / 退出码 |
|---|---|---|
| 配置加载/校验失败 | 启动期 | exit 2，stderr 给出具体字段 |
| args 解析失败（缺 platform、output 不被支持） | 启动期 | exit 2，stderr 给修正建议 |
| platform 在 config 中缺失 | 启动期 | exit 2，列出缺失列表 |
| buildctl / manifest-tool 二进制找不到 | 启动期 | exit 127 |
| buildctl 子进程失败 | 构建期 | ctx.Cancel()，其他子进程 SIGTERM；**不调用 manifest-tool**；退出码为首个失败 buildctl 的退出码 |
| manifest-tool 失败 | 合并期 | manifest-tool 退出码透传；per-arch 镜像保留在 registry（不自动清理） |
| buildmux 被 Ctrl-C | 任意时刻 | 转发 SIGTERM 给活动子进程，正常 cleanup，exit 130 |

总原则：buildctl/manifest-tool 的原始退出码透传；buildmux 自身校验失败用 exit 2。

## 11. 测试策略

### 单元测试（无外部依赖、CI 默认运行）

| 包 | 重点 |
|---|---|
| `internal/config` | 表驱动：合法 YAML、缺字段、TLS 三件套残缺、tag 模板语法错；tag 模板渲染上下文（`.Name`/`.Repo`/`.Tag`/`.OS`/`.Arch`/`.Variant`） |
| `internal/cli/args` | 表驱动：各种 `--opt platform` 写法（等号/空格/重复出现/多值）；`--output` 解析；Rest 是否干净；启动期错误集 |
| `internal/output` | KV 字符串 round-trip：解析 → 改 name → 序列化，不丢字段、不乱序、转义正确 |
| `internal/cli/build` | 通过 fake Runner interface 验证：每个 platform 用对了 endpoint/TLS flag/重写后的 name；manifest-tool 命令的 `--platforms`/`--template`/`--target` 都对 |
| `internal/dispatch` | fake 任务：全成功；首个失败 ctx 取消；signal 转发 |

`internal/exec` 不写单元测试（薄壳，靠集成测试覆盖）。

### 集成测试（`//go:build integration`，CI 可选阶段）

在 `testdata/` 用 Go 编译两个桩二进制 `fake-buildctl`、`fake-manifest-tool`，按传入 args 打印断言锚点 + 用 env var 控制是否失败。通过 `--buildctl`/`--manifest-tool` flag 指向这两个桩，端到端跑通：

- 多平台成功路径
- 一个平台失败 → 其他被取消、manifest-tool 不被调用
- platform 不在 config → 启动期报错

### 不写的测试

- 不起真 buildkitd，不连真 registry（属于 buildctl 的责任范围）
- 不验证 manifest list 的内部字节（属于 manifest-tool 的责任范围）
