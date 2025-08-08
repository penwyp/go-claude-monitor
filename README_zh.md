# go-claude-monitor

一个用于监控和分析 Claude Code 使用情况的命令行工具，提供详细的成本分析和实时会话跟踪。

[English Documentation](./README.md)

## 功能特性

- 📊 **使用分析**：分析 Claude Code 使用情况，提供详细的 Token 和成本明细
- 🔄 **实时监控**：类似 Linux `top` 命令的实时仪表板
- 💰 **成本跟踪**：按模型、项目和时间段跟踪成本
- 📈 **会话检测**：自动检测 5 小时会话窗口
- 🚀 **高性能**：并发处理与智能缓存

## 🚀 安装

### 安装方式

#### 使用 Homebrew (macOS/Linux)

```bash
brew tap penwyp/go-claude-monitor
brew install go-claude-monitor
```

#### 使用 Go

```bash
go install github.com/penwyp/go-claude-monitor@latest
```

#### 下载二进制文件

从 [GitHub Releases](https://github.com/penwyp/go-claude-monitor/releases) 下载适合您平台的最新版本。

#### 验证安装

```bash
go-claude-monitor --version
```

## 快速开始

### 基础使用分析

```bash
# 使用默认设置分析所有使用情况
go-claude-monitor

# 分析最近 7 天并显示成本明细
go-claude-monitor --duration 7d --breakdown

# 输出为 JSON 格式
go-claude-monitor --output json

# 清除缓存重新分析
go-claude-monitor --reset
```

### 实时监控

```bash
# 使用默认设置监控
go-claude-monitor top

# 使用特定套餐限制监控
go-claude-monitor top --plan max5

# 使用特定时区
go-claude-monitor top --timezone Asia/Shanghai

```

## 命令选项

### 分析命令（默认）

| 选项            | 简写   | 描述                                 | 默认值                  |
|---------------|------|------------------------------------|----------------------|
| `--dir`       |      | Claude 项目目录                        | `~/.claude/projects` |
| `--duration`  | `-d` | 时间范围（如 7d、2w、1m）                   | 所有时间                 |
| `--output`    | `-o` | 输出格式（table、json、csv、summary）       | `table`              |
| `--breakdown` | `-b` | 显示模型成本明细                           | `false`              |
| `--group-by`  |      | 分组方式（model、project、day、week、month） | `day`                |
| `--timezone`  |      | 时区（如 UTC、Asia/Shanghai）            | `Local`              |

### Top 命令

| 选项               | 描述                          | 默认值      |
|------------------|-----------------------------|----------|
| `--plan`         | 套餐类型（pro、max5、max20、custom） | `custom` |
| `--refresh-rate` | 数据刷新间隔（秒）                   | `10`     |
| `--timezone`     | 时区设置                        | `Local`  |

## 使用示例

### 基于时间的分析

```bash
# 最近 24 小时
go-claude-monitor --duration 24h

# 最近一周
go-claude-monitor --duration 7d

# 最近一个月，按天分组
go-claude-monitor --duration 1m --group-by day

```

### 输出格式

```bash
# 表格格式（默认）
go-claude-monitor

# JSON 格式，用于程序化处理
go-claude-monitor --output json > usage.json

# CSV 格式，用于电子表格
go-claude-monitor --output csv > usage.csv

# 仅显示摘要
go-claude-monitor --output summary
```

### 分组和排序

```bash
# 按模型分组
go-claude-monitor --group-by model

# 按项目分组
go-claude-monitor --group-by project

```

## 会话窗口

Claude Code 使用 5 小时会话窗口。本工具自动检测会话边界，使用以下方法：

- 🎯 **限制消息**：来自 Claude 的限制提示
- ⏳ **时间间隔**：大于 5 小时的间隔
- 📍 **首条消息**：时间戳
- ⚪ **小时对齐**：后备方案

## 开发

```bash
# 运行测试
make test

# 格式化代码
make fmt

# 运行代码检查
make lint

# 生成覆盖率报告
make coverage
```

## 许可证

MIT 许可证

## 作者

[penwyp](https://github.com/penwyp)