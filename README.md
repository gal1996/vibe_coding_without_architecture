# Vibe-Coding Without Architecture

Vibe-Codingの有効性検証実験プロジェクト（グループA: No-Arch）

## 概要

このリポジトリは、ソフトウェア開発手法「Vibe-Coding」において、アーキテクチャ指示の有無が開発成果に与える影響を検証する実験プロジェクトです。本リポジトリは**アーキテクチャ指示なし**で実装を行うグループAの成果物です。

## 実装内容

ECサイトバックエンドAPIをGo言語で実装しています。意図的にシンプルな設計を採用し、単一ファイルですべての機能を実装しています。

### 主な機能

- **商品管理**
  - 商品一覧表示（カテゴリフィルタ対応）
  - 商品詳細表示
  - 商品作成（管理者権限必須）

- **ユーザー管理**
  - ユーザー登録
  - ログイン認証（BCryptによるパスワードハッシュ化）
  - トークンベース認証

- **注文処理**
  - 注文作成（在庫チェック・自動減算）
  - 注文一覧表示
  - 消費税（10%）計算
  - 送料計算（5,000円以上で送料無料、未満は500円）

## セットアップ

### 必要な環境

- Go 1.21以上
- Git

### インストール

```bash
# リポジトリのクローン
git clone https://github.com/gal1996/vibe_coding_without_architecture.git
cd vibe_coding_without_architecture

# 依存関係のインストール
go mod download

# ビルド
go build -o ec-backend
```

## 使い方

### サーバーの起動

```bash
# 直接実行
go run main.go

# またはビルド済みバイナリを実行
./ec-backend
```

サーバーはポート8081で起動します。

### デフォルト管理者アカウント

- Username: `admin`
- Password: `admin123`

### APIエンドポイント

| メソッド | エンドポイント | 説明 | 認証 |
|---------|---------------|------|------|
| GET | `/products` | 商品一覧取得（`?category=xxx`でフィルタ可能） | 不要 |
| GET | `/products/{id}` | 商品詳細取得 | 不要 |
| POST | `/products` | 商品作成 | 管理者のみ |
| POST | `/register` | ユーザー登録 | 不要 |
| POST | `/login` | ログイン | 不要 |
| POST | `/orders` | 注文作成 | 要認証 |
| GET | `/orders` | 注文一覧取得（自分の注文のみ） | 要認証 |

### 認証方法

ログインまたは登録後に取得したトークンを、Authorizationヘッダーに設定：

```bash
Authorization: Bearer {token}
```

## テスト

### 単体テストの実行

```bash
# すべてのテストを実行
go test -v

# カバレッジ付きでテスト実行
go test -v -cover

# カバレッジレポートをHTMLで出力
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### 統合テストの実行

```bash
# APIテストスクリプトを実行（jqが必要）
./test_api.sh
```

## プロジェクト構成

```
.
├── main.go              # メインアプリケーション（すべての実装）
├── main_test.go         # 単体テスト
├── test_api.sh          # API統合テストスクリプト
├── go.mod               # Goモジュール定義
├── go.sum               # 依存関係のチェックサム
├── experiment_log.csv   # 実験データ記録
├── prompts_log.md       # プロンプト履歴
├── measure_tools.sh     # LOC計測ツール
├── CLAUDE.md            # 実験詳細説明
└── README.md            # このファイル
```

## 実験について

このプロジェクトは、Vibe-Codingにおけるアーキテクチャ指示の影響を検証する学術的実験の一部です。詳細は[CLAUDE.md](./CLAUDE.md)を参照してください。

### 測定項目

- 実行時間 (T_exec)
- コード品質 (Q_dev)
- 保守性 (T_maint, LOC_churn)
- 欠陥発生数 (Defects)
- 構造理解度 (S_under)

## 技術的特徴

- **言語**: Go 1.21
- **データストア**: インメモリ（永続化なし）
- **認証**: BCrypt + トークンベース
- **アーキテクチャ**: モノリシック（単一ファイル実装）
- **外部依存**: golang.org/x/crypto のみ

## ライセンス

このプロジェクトは実験用のサンプルコードです。

## 注意事項

- このコードは実験用であり、本番環境での使用は想定していません
- データはメモリ上にのみ保存され、サーバー再起動時にリセットされます
- セキュリティ機能は基本的なもののみ実装されています