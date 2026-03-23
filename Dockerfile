# 使用官方的Go镜像作为构建环境
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制go.mod和go.sum文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN go build -o okx-quant ./cmd/main.go

# 使用轻量级的Alpine镜像作为运行环境
FROM alpine:3.18

# 设置工作目录
WORKDIR /app

# 复制构建好的应用
COPY --from=builder /app/okx-quant .

# 复制配置文件
COPY configs/ /app/configs/

# 安装必要的依赖
RUN apk add --no-cache ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 暴露端口（如果需要）
EXPOSE 8080

# 运行应用
CMD ["./okx-quant"]
