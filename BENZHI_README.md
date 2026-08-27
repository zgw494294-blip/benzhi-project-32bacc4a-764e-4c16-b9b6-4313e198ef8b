# BENZHI_README

基于 Go 实现的野迹裁定台 HTTP API 项目，一款后端服务，供野外生态调查员与复核员完成红外相机记录从建案、争议裁定到发布。

## 项目说明
- 项目：benzhi-project-32bacc4a-764e-4c16-b9b6-4313e198ef8b
- 项目用途：供野外生态调查员与复核员完成红外相机记录从建案、争议裁定到发布。
- Go 工具链：`golang:1.23`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/trapreview -selfcheck -selfcheck-timeout=15s -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-32bacc4a-764e-4c16-b9b6-4313e198ef8b-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-32bacc4a-764e-4c16-b9b6-4313e198ef8b-arm64 linux/arm64
docker run -it benzhi-project-32bacc4a-764e-4c16-b9b6-4313e198ef8b-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/trapreview -selfcheck -selfcheck-timeout=15s -addr=127.0.0.1:19081`
