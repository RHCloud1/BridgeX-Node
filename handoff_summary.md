# BridgeX 对话接管摘要（中文）

## 当前结论
1. 你想要的方向是“可控、一键部署、兼容 V2Board/XBoard + 多内核（sing-box/xray/hysteria2/anytls）”的项目。
2. 已有项目 `nodebridge` 由我搭建，目前定位为 Go 控制面服务（非核心协议实现本体）。
3. 你希望把项目转到 `D:\Project\AI\BridgeX Node` 下继续开发，我已完成迁移。

## 迁移结果
- 旧路径：`D:\Project\Go\nodebridge`
- 新路径：`D:\Project\AI\BridgeX Node`
- 旧路径已清理（原仓库目录），新路径已含全部文件。

## 项目当前能力（截至本对话）
- 支持的订阅/节点格式：vmess、vless、trojan、shadowsocks、hysteria、hysteria2、tuic、anytls
- 面板同步：v2board/xboard 兼容逻辑在 `internal/panel` 与 `internal/subscription`
- 渲染：sing-box / xray / hysteria2 基础配置渲染已实现
- 同步流程：`internal/service` 周期抓取并写入运行时 registry
- 一键部署：`deploy/install.sh` 可下载 release、生成基础 `config.json`、拉取核心（二进制）并写 systemd
- 管理脚本：`deploy/nodebridge.sh`

## 与 RHCloud1/RHCloud-V2BX 的对比结论
- RHCloud 的安装体验成熟，但维护上更偏“脚本拼配置”模式，sing-box 兼容问题常见于配置模板与版本变化。
- 你当前项目的优势是“配置链路可控”：核心不内嵌在脚本里，而是在 Go 侧统一模型后渲染，便于长期维护。

## 重要日期与事实
- 本地项目已有两个提交：初始 scaffold 与面板渲染/一键安装增强。
- 已在本地做过发布包打包（含 Linux amd64/arm64、windows amd64）。

## 建议的命名
- 已建议名称：
  - BridgeX Node（推荐）
  - NodePilot
  - AnyCore Proxy
  - PanelNode Bridge
  - V2Core Control

## 接下来建议（你可直接继续）
1. 在新目录继续开发：`D:\Project\AI\BridgeX Node`
2. 先把仓库重命名/远程改为正式名字（可用 `BridgeX` 体系）
3. 继续补齐 RHCloud 风格的运维体验（版本提示、update 命令、配置补全提示），并把 AnyTLS 导出配置补齐。
