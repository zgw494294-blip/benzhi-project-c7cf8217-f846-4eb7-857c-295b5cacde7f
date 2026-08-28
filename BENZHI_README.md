# BENZHI_README

基于 Go 实现的古籍数字校勘入藏 Web 项目，一款后端服务，用于管理扫描页编排、逐页转录、疑难字校勘和数字版本入藏。

## 项目说明
- 项目：benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f
- 项目用途：古籍数字校勘入藏台已完整实现。项目提供分层 Go 服务、SQLite 持久化、原生浏览器工作台和同源 JSON API，贯通建卷、扫描页编排、转录、疑难处理、完整性检查、版本冻结及入藏复核。最终回归测试和真实 HTTP 自检均已通过。
- Go 工具链：`golang:1.24.0`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selfcheck -addr=127.0.0.1:19081 -data=:memory:
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f-arm64 linux/arm64
docker run -it benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selfcheck -addr=127.0.0.1:19081 -data=:memory:`
