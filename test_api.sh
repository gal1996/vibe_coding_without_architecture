#!/bin/bash

# APIテストスクリプト
BASE_URL="http://localhost:8081"

echo "===== ECサイトバックエンドAPI テスト ====="
echo ""

# 1. 商品一覧取得
echo "1. 商品一覧取得"
curl -s "$BASE_URL/products" | jq '.'
echo ""

# 2. カテゴリフィルタで商品一覧取得
echo "2. 電子機器カテゴリの商品のみ取得"
curl -s "$BASE_URL/products?category=電子機器" | jq '.'
echo ""

# 3. 商品詳細取得
echo "3. 商品詳細取得 (ID=1)"
curl -s "$BASE_URL/products/1" | jq '.'
echo ""

# 4. ユーザー登録
echo "4. 新規ユーザー登録"
REGISTER_RESPONSE=$(curl -s -X POST "$BASE_URL/register" \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "password": "test123"}')
echo "$REGISTER_RESPONSE" | jq '.'
USER_TOKEN=$(echo "$REGISTER_RESPONSE" | jq -r '.token')
echo ""

# 5. ログイン
echo "5. ユーザーログイン"
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "password": "test123"}')
echo "$LOGIN_RESPONSE" | jq '.'
echo ""

# 6. 管理者ログイン
echo "6. 管理者ログイン"
ADMIN_RESPONSE=$(curl -s -X POST "$BASE_URL/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin123"}')
echo "$ADMIN_RESPONSE" | jq '.'
ADMIN_TOKEN=$(echo "$ADMIN_RESPONSE" | jq -r '.token')
echo ""

# 7. 商品作成（管理者のみ）
echo "7. 新商品作成（管理者権限）"
curl -s -X POST "$BASE_URL/products" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "name": "キーボード",
    "price": 8000,
    "stock": 20,
    "category": "電子機器"
  }' | jq '.'
echo ""

# 8. 商品作成（一般ユーザーでは失敗）
echo "8. 新商品作成（一般ユーザーでは失敗するはず）"
curl -s -X POST "$BASE_URL/products" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{
    "name": "モニター",
    "price": 40000,
    "stock": 5,
    "category": "電子機器"
  }' | jq '.'
echo ""

# 9. 注文作成（5000円以上で送料無料）
echo "9. 注文作成（5000円以上で送料無料）"
echo "   商品1: ノートPC 120,000円 x 1 = 120,000円"
echo "   商品合計（税抜）: 120,000円"
echo "   消費税（10%）: 12,000円"
echo "   送料: 0円（5000円以上）"
echo "   予想合計: 132,000円"
curl -s -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{
    "items": [
      {"product_id": 1, "quantity": 1}
    ]
  }' | jq '.'
echo ""

# 9-2. 注文作成（5000円未満で送料500円）
echo "9-2. 注文作成（5000円未満で送料500円）"
echo "   商品2: マウス 3,000円 x 1 = 3,000円"
echo "   商品合計（税抜）: 3,000円"
echo "   消費税（10%）: 300円"
echo "   送料: 500円（5000円未満）"
echo "   予想合計: 3,800円"
curl -s -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{
    "items": [
      {"product_id": 2, "quantity": 1}
    ]
  }' | jq '.'
echo ""

# 10. 在庫不足の注文（失敗するはず）
echo "10. 在庫不足の注文（失敗するはず）"
curl -s -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{
    "items": [
      {"product_id": 3, "quantity": 100}
    ]
  }' | jq '.'
echo ""

# 11. ユーザーの注文履歴取得
echo "11. ユーザーの注文履歴取得"
curl -s "$BASE_URL/orders" \
  -H "Authorization: Bearer $USER_TOKEN" | jq '.'
echo ""

# 12. 更新後の商品一覧（在庫が減っているはず）
echo "12. 更新後の商品一覧（在庫が減っているはず）"
curl -s "$BASE_URL/products" | jq '.'
echo ""

echo "===== テスト完了 ====="