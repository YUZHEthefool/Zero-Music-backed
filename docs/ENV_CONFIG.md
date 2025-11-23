# 环境变量配置说明

Zero-Music 后端支持通过环境变量覆盖配置文件中的设置。环境变量的优先级高于配置文件。

## 可用的环境变量

### 服务器配置

| 环境变量 | 说明 | 默认值 | 示例 |
|---------|------|--------|------|
| `ZERO_MUSIC_SERVER_HOST` | 服务器监听地址 | `0.0.0.0` | `ZERO_MUSIC_SERVER_HOST=127.0.0.1` |
| `ZERO_MUSIC_SERVER_PORT` | 服务器监听端口 | `8080` | `ZERO_MUSIC_SERVER_PORT=3000` |
| `ZERO_MUSIC_MAX_RANGE_SIZE` | 单次 Range 请求最大字节数 | `104857600` (100MB) | `ZERO_MUSIC_MAX_RANGE_SIZE=52428800` |

### 音乐库配置

| 环境变量 | 说明 | 默认值 | 示例 |
|---------|------|--------|------|
| `ZERO_MUSIC_MUSIC_DIRECTORY` | 音乐文件目录 | `~/Music` 或 `./music` | `ZERO_MUSIC_MUSIC_DIRECTORY=/data/music` |
| `ZERO_MUSIC_CACHE_TTL_MINUTES` | 缓存有效期（分钟） | `5` | `ZERO_MUSIC_CACHE_TTL_MINUTES=10` |

## 使用方法

### 方法一：直接设置环境变量

```bash
# Linux/macOS
export ZERO_MUSIC_SERVER_PORT=3000
export ZERO_MUSIC_MUSIC_DIRECTORY=/path/to/music
./zero-music

# Windows (PowerShell)
$env:ZERO_MUSIC_SERVER_PORT=3000
$env:ZERO_MUSIC_MUSIC_DIRECTORY="C:\Music"
.\zero-music.exe
```

### 方法二：使用 .env 文件（配合第三方工具）

1. 复制示例文件：
```bash
cp .env.example .env
```

2. 编辑 `.env` 文件，设置您的配置

3. 使用支持 .env 文件的工具运行（如 `godotenv`）：
```bash
# 安装 godotenv
go install github.com/joho/godotenv/cmd/godotenv@latest

# 运行
godotenv -f .env ./zero-music
```

### 方法三：Docker 环境

```bash
docker run -d \
  -e ZERO_MUSIC_SERVER_PORT=3000 \
  -e ZERO_MUSIC_MUSIC_DIRECTORY=/music \
  -v /path/to/music:/music \
  -p 3000:3000 \
  zero-music
```

## 配置优先级

配置的加载优先级从高到低为：

1. 环境变量
2. 配置文件 (`config.json`)
3. 默认值

例如，如果同时在配置文件中设置了 `port: 8080`，又设置了环境变量 `ZERO_MUSIC_SERVER_PORT=3000`，则最终使用的端口是 `3000`。

## 验证配置

启动服务器时，日志会显示实际使用的配置：

```
{"level":"info","msg":"配置加载成功: 服务器地址=0.0.0.0:3000, 音乐目录=/data/music","time":"2025-11-23 02:12:00"}
```

## 注意事项

1. 环境变量值必须符合类型要求（如端口号必须是 1-65535 之间的整数）
2. 如果环境变量值格式不正确，将使用配置文件中的值或默认值
3. `MUSIC_DIRECTORY` 支持相对路径和绝对路径
4. 建议在生产环境中使用环境变量管理敏感配置
