# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# 安装依赖
RUN apk add --no-cache git

# 复制 go mod 文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o funnyai-server .

# Run stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 从 builder 复制二进制
COPY --from=builder /app/funnyai-server .

# 创建上传目录
RUN mkdir -p /app/uploads

EXPOSE 8080

CMD ["./funnyai-server"]
