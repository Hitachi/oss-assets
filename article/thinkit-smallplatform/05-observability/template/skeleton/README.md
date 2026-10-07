# Template App

Go言語とEchoフレームワークで構築されたシンプルなREST APIテンプレートアプリケーションです。OpenAPI 3.0仕様とoapi-codegenを使用してコード生成を行います。

## 特徴

- OpenAPI 3.0仕様
- oapi-codegenによる自動コード生成
- golangci-lintによるコード品質チェック
- goimportsによるコードフォーマット
- Github ActionsによるCI
  - コード品質チェック
  - ユニットテスト
  - コンテナビルド、プッシュ
  - Trivyによるセキュリティスキャン
- KustomizeによるKubernetesマニフェスト管理

## APIエンドポイント

- **GET** `/health` - サービスが正常な場合204を返す
- **GET** `/hello` - hello メッセージを返す

## はじめに

### 前提条件

- Go 1.25.1 以降
- Make
- Visual Studio Code用の設定が含まれるため、IDEとしてVisual Studio Codeを推奨

### インストール

依存関係をインストール:
```bash
go mod download
```

### 開発

#### コード生成:
```bash
make generate
```

#### 実行:
```bash
make dev
```

#### フォーマット:
```bash
make fmt
```

#### 静的コード解析:
```bash
make lint
```

#### ユニットテスト:
```bash
make test
```

#### ビルド
```bash
make build
```

#### ビルド成果物をクリーンアップ:
```bash
make clean
```

#### コンテナビルド
```bash
make docker-build
```

#### コンテナのセキュリティスキャン
```bash
make trivy-scan
```

### APIのテスト

サーバーが `http://localhost:8080` で実行されている状態で、エンドポイントをテストできます：

#### ヘルスチェック
```bash
curl -i http://localhost:8080/health
```

期待されるレスポンス:
```
HTTP/1.1 204 No Content
```

#### Hello エンドポイント
```bash
curl -i http://localhost:8080/hello
```

期待されるレスポンス:
```
HTTP/1.1 200 OK
Content-Type: application/json

{"message":"hello"}
```

## プロジェクト構造

```
.
├── .golangci.yaml          # golangci-lint設定
├── Dockerfile              # Docker設定
├── Makefile                # ビルド・開発コマンド
├── main.go                 # アプリケーションのエントリーポイント
├── gen.go                  # コード生成用エントリーポイント
├── oapi-codegen.yaml       # oapi-codegen設定
├── openapi.yaml            # OpenAPI 3.0仕様
├── deployments/            # デプロイメント関連ファイル
│   ├── README.md           # デプロイメント手順
│   └── kustomize/          # Kubernetesマニフェスト管理
│       ├── base/           # ベースマニフェスト
│       │   ├── configmap.yaml
│       │   ├── deployment.yaml
│       │   ├── httproute.yaml
│       │   ├── kustomization.yaml
│       │   └── service.yaml
│       └── overlays/       # 環境別設定
│           └── production/ # 本番環境設定
│               ├── authpolicy.yaml
│               ├── hpa.yaml
│               ├── kustomization.yaml
│               ├── network-policy.yaml
│               └── pdb.yaml
└── internal/
    └── api/
        ├── api.gen.go      # 生成されたAPIインターフェース（編集禁止）
        ├── server.go       # API実装
        └── server_test.go  # APIのテスト
```

## OpenAPI仕様

APIは `openapi.yaml` でOpenAPI 3.0仕様に従って定義されています。サーバーインターフェースはoapi-codegenを使用して自動生成されます。

APIを変更する場合：
1. `openapi.yaml` を編集
2. `make generate` を実行してコードを再生成
3. `internal/api/server.go` の実装を更新

## ライセンス

このプロジェクトはApache 2.0ライセンスの下でライセンスされています。詳細は[LICENSE](LICENSE)ファイルを参照してください。
