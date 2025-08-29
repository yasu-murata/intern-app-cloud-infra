# ---- ビルドステージ ----
# Goのビルド環境として、軽量なalpineバージョンのgolangイメージを使用します
FROM golang:1.23-alpine AS builder

# 作業ディレクトリを設定
WORKDIR /app

# Goモジュールの依存関係を先にコピーしてキャッシュを活用します
COPY go.mod go.sum ./
RUN go mod download

# アプリケーションの全ソースコードをコピーします
COPY . .

# アプリケーションをビルドします
# CGO_ENABLED=0: C言語のライブラリに依存しない静的なバイナリを生成
# GOOS=linux: 最終的な実行環境であるLinux向けのバイナリを生成
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# ---- 実行ステージ ----
# 実行環境として、Googleが提供する超軽量・セキュアなDistrolessイメージを使用します
FROM gcr.io/distroless/static-debian11

# 作業ディレクトリを設定
WORKDIR /

# ビルドステージからコンパイル済みのバイナリのみをコピーします
COPY --from=builder /app/main .

# コンテナ起動時に実行するコマンドを指定します
ENTRYPOINT ["/main"]