# agy-usage

[English](./README.md) | **简体中文**

专为 **Google Antigravity**（同时覆盖 **Antigravity IDE/桌面端** 与 **Antigravity CLI**）打造的轻量、零依赖命令行与本地 Web **Token 消耗看板工具**，提供类似 Codex / `ccusage` 的即开即看体验。

全仓采用 **100% Go 标准库纯净实现（`CGO_ENABLED=0`）**，内置只读 SQLite3 B-Tree + WAL 解析器，无需安装 Python、无需 CGO 或任何第三方依赖包，在 Linux / macOS / Windows 上 1 秒即可编译为独立单文件二进制。

---

## 核心特性

- **全端自动汇总**：跨平台自动扫描 `~/.gemini/antigravity`（桌面端 / IDE）与 `~/.gemini/antigravity-cli`（命令行 `agy`）的全部会话日志与元数据。
- **精准口径拆解**：清晰区分 **未缓存新增输入（Uncached Input）**、**缓存读取（Cache Read + 命中率）**、**输出（Output）**、**非缓存净消耗（Net = In + Out）** 与 **含缓存总吞吐（Total）**。
- **Subagent 子代理自动归集**：解析 `conversation_summaries.db` 中的父子会话树，默认将派生的子智能体开销汇总至父会话与对应项目（显示 `[+N子]` 标记），也支持 `--flat` 平铺查看每个子代理。
- **上下文自动压缩识别**：自动识别 `CHECKPOINT` 系统摘要步骤，单独统计为 `[context-compaction]` 及触发次数。
- **无标题 CLI 会话补全**：针对默认无标题的 CLI 会话，自动提取首条 `<USER_REQUEST>` 内容作为可读标题。
- **终端实时盯盘 + 本地暗黑 Web 看板**：
  - `agy-usage -w`：支持分屏无闪烁实时刷新盯盘，跑 `/goal` 或多代理任务时实时观察 Token 变化。
  - `agy-usage --web`：单二进制内置暗色可视化 Web 看板（默认 `http://127.0.0.1:8765`），支持动态过滤、搜索与中英一键切换。
- **100% 本地只读离线运行**：纯只读扫描本地日志，不修改任何数据，不发起任何外部网络请求。

---

## 终端看板预览

```text
╭──────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ ◆ ANTIGRAVITY TOKEN DASHBOARD [ALL TIME]                                                 2026-11-25 16:30:47 │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ 总消耗 (含缓存)          │ 非缓存净消耗             │ 缓存读取 (Cache)         │ 模型总输出                  │
│ 737.30M                  │ 158.59M                  │ 578.71M                  │ 4.70M                       │
│ 737,297,908              │ In 153.89M+Out 4.70M     │ 命中率 79.0%             │ 占比 0.6%                   │
│ ──────────────────────────────────────────────────────────────────────────────────────────────────────────── │
│ 会话总数                 │ 模型调用步数             │ 子智能体消耗             │ 日均总吞吐                  │
│ 43                       │ 3,129                    │ 74.37M                   │ 184.32M                     │
│ 主 17 · 子代理 26        │ 自动压缩 17 次           │ 占比 10.1% (487步)       │ 活跃 4 天                   │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ▸ 每日消耗趋势 (Daily Trend)                                                      █ 非缓存净消耗  ▒ 缓存读取 │
│ 日期          步数   未缓存In   缓存Read   输出Out   当日总计  趋势                                          │
│ 2026-11-23    1125     98.40M    217.50M     2.10M    318.00M  ██████████████▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒ │
│ 2026-11-24    1236     32.87M    182.77M     1.35M    216.99M  █████▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒░░░░░░░░░░░░░░ │
│ 2026-11-25*    768     22.62M    178.44M     1.25M    202.31M  ████▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒░░░░░░░░░░░░░░░░ │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

---

## 安装方式

### 方式一：`go install` 一键安装（推荐，Go 1.22+）

零外部依赖，无需代理下载第三方库，秒级编译：

```bash
CGO_ENABLED=0 go install github.com/LiangNing7/agy-usage/cmd/agy-usage@latest
```

### 方式二：安装脚本（Linux / macOS）

```bash
curl -fsSL https://raw.githubusercontent.com/LiangNing7/agy-usage/main/install.sh | bash
```

### 方式三：源码编译

```bash
git clone https://github.com/LiangNing7/agy-usage.git
cd agy-usage
make install   # 默认安装至 ~/.local/bin/agy-usage
```

---

## 常用命令速查

```bash
# 1. 默认完整终端彩色看板（自动根据系统语言切换中文/英文）
agy-usage

# 2. 实时刷新盯盘模式（默认每 2 秒刷新，也可指定秒数）
agy-usage -w
agy-usage -w 5

# 3. 启动本地交互式 Web 网页看板（默认 http://127.0.0.1:8765）
agy-usage --web
agy-usage --web --port 9000 --no-open

# 4. 时间与多维度筛选
agy-usage -t                        # 仅查看今日消耗 (--today)
agy-usage -d 7                      # 查看最近 7 天消耗 (--days 7)
agy-usage --since 2026-11-01        # 指定起止日期 (--until YYYY-MM-DD)
agy-usage -s cli                    # 按端侧过滤: all | ide | cli
agy-usage -p nora                   # 按工作区项目名或会话标题过滤
agy-usage -m carbon                 # 按模型名称过滤

# 5. 视图控制与 JSON 导出
agy-usage --sort recent --top 20    # 会话排序: tokens (默认) | recent | steps
agy-usage --flat                    # 展开所有 Subagent 子会话（不向父会话折叠）
agy-usage --raw                     # 显示精确千分位整数（而非 K/M/B 单位）
agy-usage --lang zh                 # 手动指定界面语言: auto | zh | en
agy-usage --json                    # 导出结构化 JSON（便于接入脚本或状态栏）
```

---

## 数据目录与环境变量

`agy-usage` 默认读取以下官方数据目录，也支持通过环境变量或参数覆盖（例如分析备份目录）：

| 端侧 | Linux / macOS 默认路径 | Windows 默认路径 | 环境变量覆盖 | 参数覆盖 |
| :--- | :--- | :--- | :--- | :--- |
| **Antigravity IDE / 桌面端** | `~/.gemini/antigravity` | `%USERPROFILE%\.gemini\antigravity` | `AGY_DATA_DIR` | `--agy-dir` |
| **Antigravity CLI (`agy`)** | `~/.gemini/antigravity-cli` | `%USERPROFILE%\.gemini\antigravity-cli` | `AGY_CLI_DATA_DIR` | `--cli-dir` |

其他环境变量：
- `AGY_LANG=zh|en`：固定默认界面语言。
- `NO_COLOR=1`：禁用终端 ANSI 颜色输出。

---

## License

[MIT](./LICENSE) © 2026 LiangNing7
