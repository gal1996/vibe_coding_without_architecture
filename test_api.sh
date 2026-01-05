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

echo "===== クーポンシステムテスト ====="
echo ""

# 13. 固定額クーポンのテスト（FLAT1000）
echo "13. 固定額クーポンテスト（FLAT1000: 1000円割引）"
echo "   商品1: ノートPC 120,000円 x 1 = 120,000円"
echo "   商品合計（税抜）: 120,000円"
echo "   消費税（10%）: 12,000円"
echo "   送料: 0円（5000円以上）"
echo "   クーポン割引: 1,000円"
echo "   予想合計: 131,000円"
curl -s -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{
    "items": [
      {"product_id": 1, "quantity": 1}
    ],
    "coupon_code": "FLAT1000"
  }' | jq '.
  | {
    order_id: .id,
    total_price: .total_price,
    shipping_fee: .shipping_fee,
    discount_amount: .discount_amount,
    applied_coupon: .applied_coupon,
    status: .status,
    transaction_id: .transaction_id
  }'
echo ""

# 14. パーセンテージクーポンのテスト（SAVE20）
echo "14. パーセンテージクーポンテスト（SAVE20: 20%割引）"
echo "   商品2: マウス 3,000円 x 1 = 3,000円"
echo "   商品合計（税抜）: 3,000円"
echo "   消費税（10%）: 300円"
echo "   小計: 3,300円"
echo "   送料: 500円（5000円未満）"
echo "   クーポン割引: 660円（3,300円の20%）"
echo "   予想合計: 3,140円"
curl -s -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{
    "items": [
      {"product_id": 2, "quantity": 1}
    ],
    "coupon_code": "SAVE20"
  }' | jq '.
  | {
    order_id: .id,
    total_price: .total_price,
    shipping_fee: .shipping_fee,
    discount_amount: .discount_amount,
    applied_coupon: .applied_coupon,
    status: .status,
    transaction_id: .transaction_id
  }'
echo ""

# 15. 無効なクーポンコードのテスト
echo "15. 無効なクーポンコードのテスト"
curl -s -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{
    "items": [
      {"product_id": 2, "quantity": 1}
    ],
    "coupon_code": "INVALID_COUPON"
  }' | jq '.'
echo ""

# 16. 複数商品での高額クーポンテスト（FLAT2000）
echo "16. 複数商品での高額クーポンテスト（FLAT2000: 2000円割引）"
echo "   商品3: デスク 25,000円 x 1 = 25,000円"
echo "   商品4: チェア 15,000円 x 1 = 15,000円"
echo "   商品合計（税抜）: 40,000円"
echo "   消費税（10%）: 4,000円"
echo "   送料: 0円（5000円以上）"
echo "   クーポン割引: 2,000円"
echo "   予想合計: 42,000円"
curl -s -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{
    "items": [
      {"product_id": 3, "quantity": 1},
      {"product_id": 4, "quantity": 1}
    ],
    "coupon_code": "FLAT2000"
  }' | jq '.
  | {
    order_id: .id,
    total_price: .total_price,
    shipping_fee: .shipping_fee,
    discount_amount: .discount_amount,
    applied_coupon: .applied_coupon,
    status: .status,
    transaction_id: .transaction_id
  }'
echo ""

echo "===== 決済処理テスト ====="
echo "17. 決済シミュレーション（90%の確率で成功）"
echo "   複数回注文を試みて、決済の成功/失敗を確認"
echo ""

# 新しいユーザーを作成（決済テスト用）
echo "   決済テスト用の新規ユーザーを作成..."
PAYMENT_USER_RESPONSE=$(curl -s -X POST "$BASE_URL/register" \
  -H "Content-Type: application/json" \
  -d '{"username": "paymenttest_'$(date +%s)'", "password": "test123"}')
PAYMENT_TOKEN=$(echo "$PAYMENT_USER_RESPONSE" | jq -r '.token')

# 5回試行して決済の成功・失敗パターンを確認
for i in {1..5}; do
  echo ""
  echo "   試行 $i:"
  ORDER_PAYMENT_TEST=$(curl -s -X POST "$BASE_URL/orders" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $PAYMENT_TOKEN" \
    -d '{
      "items": [
        {"product_id": 2, "quantity": 1}
      ]
    }')

  # レスポンスの確認
  if echo "$ORDER_PAYMENT_TEST" | jq -e '.status == "completed"' > /dev/null 2>&1; then
    TRANSACTION_ID=$(echo "$ORDER_PAYMENT_TEST" | jq -r '.transaction_id')
    TOTAL=$(echo "$ORDER_PAYMENT_TEST" | jq -r '.total_price')
    echo "     ✅ 決済成功"
    echo "        Transaction ID: $TRANSACTION_ID"
    echo "        Total Price: ${TOTAL}円"
    echo "        Status: completed"
  elif echo "$ORDER_PAYMENT_TEST" | jq -e '.error' > /dev/null 2>&1; then
    ERROR_MSG=$(echo "$ORDER_PAYMENT_TEST" | jq -r '.error')
    echo "     ❌ 決済失敗"
    echo "        Error: $ERROR_MSG"
    echo "        Status: payment_failed"
  else
    echo "     ⚠️ 予期しないレスポンス:"
    echo "$ORDER_PAYMENT_TEST" | jq '.'
  fi

  sleep 0.5  # サーバーに負荷をかけないため少し待つ
done

echo ""

echo "===== 販売分析レポートテスト（管理者向け） ====="
echo "18. 管理者による販売分析レポート取得"
echo ""
curl -s "$BASE_URL/admin/reports/sales" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.'
echo ""

echo "19. 一般ユーザーによるレポートアクセス（失敗するはず）"
curl -s "$BASE_URL/admin/reports/sales" \
  -H "Authorization: Bearer $USER_TOKEN" | jq '.'
echo ""

echo "===== テスト完了 ====="