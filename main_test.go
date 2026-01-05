package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// テスト用モック決済ゲートウェイ
type MockPaymentGateway struct {
	shouldSucceed bool
}

func (m *MockPaymentGateway) ProcessPayment(amount int, orderID int) PaymentResult {
	if m.shouldSucceed {
		return PaymentResult{
			Success:       true,
			TransactionID: "TEST_TXN_123",
			Message:       "Test payment successful",
		}
	}
	return PaymentResult{
		Success:       false,
		TransactionID: "",
		Message:       "Test payment failed",
	}
}

func TestGetProductsHandler(t *testing.T) {
	// テスト用の商品を追加
	productMux.Lock()
	products[100] = &Product{
		ID:       100,
		Name:     "テスト商品",
		Price:    1000,
		Category: "テスト",
	}
	productMux.Unlock()

	// テスト用の在庫を追加
	stockMux.Lock()
	stocks["100-1"] = &Stock{ProductID: 100, WarehouseID: 1, Quantity: 10}
	stockMux.Unlock()

	// 全商品取得のテスト
	req := httptest.NewRequest("GET", "/products", nil)
	w := httptest.NewRecorder()
	getProductsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result []ProductDetailResponse
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) == 0 {
		t.Error("Expected products, got empty array")
	}

	// カテゴリフィルタのテスト
	req = httptest.NewRequest("GET", "/products?category=テスト", nil)
	w = httptest.NewRecorder()
	getProductsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	json.NewDecoder(w.Body).Decode(&result)
	found := false
	for _, p := range result {
		if p.Category == "テスト" && p.TotalStock == 10 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Category filter not working or stock not calculated correctly")
	}
}

func TestGetProductHandler(t *testing.T) {
	// 存在する商品のテスト
	req := httptest.NewRequest("GET", "/products/1", nil)
	w := httptest.NewRecorder()
	getProductHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var product ProductDetailResponse
	json.NewDecoder(w.Body).Decode(&product)
	if product.ID != 1 {
		t.Errorf("Expected product ID 1, got %d", product.ID)
	}
	if product.TotalStock == 0 {
		t.Error("Expected stock information, got none")
	}

	// 存在しない商品のテスト
	req = httptest.NewRequest("GET", "/products/999", nil)
	w = httptest.NewRecorder()
	getProductHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestRegisterHandler(t *testing.T) {
	// 正常な登録
	reqBody := `{"username": "testuser2", "password": "password123"}`
	req := httptest.NewRequest("POST", "/register", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	registerHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var user User
	json.NewDecoder(w.Body).Decode(&user)
	if user.Username != "testuser2" {
		t.Errorf("Expected username testuser2, got %s", user.Username)
	}

	// 重複ユーザー名のテスト
	req = httptest.NewRequest("POST", "/register", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	registerHandler(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status %d for duplicate username, got %d", http.StatusConflict, w.Code)
	}

	// 不正なリクエストのテスト
	reqBody = `{"username": "", "password": ""}`
	req = httptest.NewRequest("POST", "/register", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	registerHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for empty fields, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestLoginHandler(t *testing.T) {
	// テスト用ユーザーを作成
	hash, _ := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.DefaultCost)
	userMux.Lock()
	testUser := &User{
		ID:           999,
		Username:     "logintest",
		PasswordHash: string(hash),
		IsAdmin:      false,
	}
	users[999] = testUser
	usersByName["logintest"] = testUser
	userMux.Unlock()

	// 正常なログイン
	reqBody := `{"username": "logintest", "password": "testpass"}`
	req := httptest.NewRequest("POST", "/login", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	loginHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var user User
	json.NewDecoder(w.Body).Decode(&user)
	if user.Token == "" {
		t.Error("Expected token, got empty string")
	}

	// 不正なパスワード
	reqBody = `{"username": "logintest", "password": "wrongpass"}`
	req = httptest.NewRequest("POST", "/login", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	loginHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d for wrong password, got %d", http.StatusUnauthorized, w.Code)
	}

	// 存在しないユーザー
	reqBody = `{"username": "nonexistent", "password": "pass"}`
	req = httptest.NewRequest("POST", "/login", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	loginHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d for nonexistent user, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestCreateProductHandler(t *testing.T) {
	// 管理者トークンを設定
	adminUser := &User{ID: 1, Username: "admin", IsAdmin: true}
	adminToken := "admin-test-token"
	sessionMux.Lock()
	sessions[adminToken] = adminUser
	sessionMux.Unlock()

	// 一般ユーザートークンを設定
	regularUser := &User{ID: 2, Username: "user", IsAdmin: false}
	userToken := "user-test-token"
	sessionMux.Lock()
	sessions[userToken] = regularUser
	sessionMux.Unlock()

	// 管理者による商品作成
	reqBody := `{"name": "新商品", "price": 5000, "initial_stock": 15, "category": "テスト"}`
	req := httptest.NewRequest("POST", "/products", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	createProductHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	// 一般ユーザーによる商品作成（失敗するはず）
	req = httptest.NewRequest("POST", "/products", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	createProductHandler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d for non-admin, got %d", http.StatusForbidden, w.Code)
	}

	// 認証なしでの商品作成（失敗するはず）
	req = httptest.NewRequest("POST", "/products", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	createProductHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d for no auth, got %d", http.StatusUnauthorized, w.Code)
	}

	// 不正なデータでの商品作成
	reqBody = `{"name": "", "price": -1, "initial_stock": -1, "category": ""}`
	req = httptest.NewRequest("POST", "/products", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	createProductHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid data, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCreateOrderHandler(t *testing.T) {
	// 元の決済ゲートウェイを保存して後で復元
	originalGateway := paymentGateway
	defer func() { paymentGateway = originalGateway }()

	// テスト用ユーザーとトークンを設定
	testUser := &User{ID: 3, Username: "orderuser", IsAdmin: false}
	userToken := "order-test-token"
	sessionMux.Lock()
	sessions[userToken] = testUser
	sessionMux.Unlock()

	// テスト用商品を追加
	productMux.Lock()
	products[200] = &Product{ID: 200, Name: "商品A", Price: 1000, Category: "テスト"}
	products[201] = &Product{ID: 201, Name: "商品B", Price: 2000, Category: "テスト"}
	productMux.Unlock()

	// テスト用在庫を追加
	stockMux.Lock()
	stocks["200-1"] = &Stock{ProductID: 200, WarehouseID: 1, Quantity: 10}
	stocks["201-1"] = &Stock{ProductID: 201, WarehouseID: 1, Quantity: 5}
	stockMux.Unlock()

	// 決済成功ケースのテスト
	paymentGateway = &MockPaymentGateway{shouldSucceed: true}

	// 正常な注文（5000円未満なので送料500円が加算される）
	reqBody := `{"items": [{"product_id": 200, "quantity": 2}, {"product_id": 201, "quantity": 1}]}`
	req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	w := httptest.NewRecorder()
	createOrderHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var order Order
	json.NewDecoder(w.Body).Decode(&order)
	// 商品合計: 1000*2 + 2000*1 = 4000円
	// 消費税: 4000 * 0.1 = 400円
	// 送料: 500円（5000円未満）
	// 合計: 4000 + 400 + 500 = 4900円
	expectedTotal := 4900
	if order.TotalPrice != expectedTotal {
		t.Errorf("Expected total price %d, got %d", expectedTotal, order.TotalPrice)
	}
	if order.ShippingFee != 500 {
		t.Errorf("Expected shipping fee 500, got %d", order.ShippingFee)
	}

	// 在庫確認
	stockMux.RLock()
	if stocks["200-1"].Quantity != 8 {
		t.Errorf("Expected stock 8 for product 200 in warehouse 1, got %d", stocks["200-1"].Quantity)
	}
	if stocks["201-1"].Quantity != 4 {
		t.Errorf("Expected stock 4 for product 201 in warehouse 1, got %d", stocks["201-1"].Quantity)
	}
	stockMux.RUnlock()

	// 送料無料のケース（5000円以上）
	productMux.Lock()
	products[202] = &Product{ID: 202, Name: "商品C", Price: 3000, Category: "テスト"}
	productMux.Unlock()
	stockMux.Lock()
	stocks["202-1"] = &Stock{ProductID: 202, WarehouseID: 1, Quantity: 10}
	stockMux.Unlock()

	reqBody = `{"items": [{"product_id": 202, "quantity": 2}]}`
	req = httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	createOrderHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d for free shipping order, got %d", http.StatusCreated, w.Code)
	}

	json.NewDecoder(w.Body).Decode(&order)
	// 商品合計: 3000*2 = 6000円
	// 消費税: 6000 * 0.1 = 600円
	// 送料: 0円（5000円以上）
	// 合計: 6000 + 600 + 0 = 6600円
	expectedTotal = 6600
	if order.TotalPrice != expectedTotal {
		t.Errorf("Expected total price %d for free shipping, got %d", expectedTotal, order.TotalPrice)
	}
	if order.ShippingFee != 0 {
		t.Errorf("Expected shipping fee 0 for free shipping, got %d", order.ShippingFee)
	}

	// 在庫不足の注文
	reqBody = `{"items": [{"product_id": 200, "quantity": 100}]}`
	req = httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	createOrderHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for insufficient stock, got %d", http.StatusBadRequest, w.Code)
	}

	// 認証なしでの注文
	req = httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	createOrderHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d for no auth, got %d", http.StatusUnauthorized, w.Code)
	}

	// 空の注文
	reqBody = `{"items": []}`
	req = httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	createOrderHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for empty order, got %d", http.StatusBadRequest, w.Code)
	}

	// 不正な数量
	reqBody = `{"items": [{"product_id": 200, "quantity": 0}]}`
	req = httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	createOrderHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid quantity, got %d", http.StatusBadRequest, w.Code)
	}

	// 存在しない商品
	reqBody = `{"items": [{"product_id": 999999, "quantity": 1}]}`
	req = httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	createOrderHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d for nonexistent product, got %d", http.StatusNotFound, w.Code)
	}

	// 決済失敗ケースのテスト
	paymentGateway = &MockPaymentGateway{shouldSucceed: false}

	// 商品の在庫を復元
	productMux.Lock()
	products[203] = &Product{ID: 203, Name: "商品D", Price: 1500, Category: "テスト"}
	productMux.Unlock()
	stockMux.Lock()
	stocks["203-1"] = &Stock{ProductID: 203, WarehouseID: 1, Quantity: 10}
	initialStock := stocks["203-1"].Quantity
	stockMux.Unlock()

	reqBody = `{"items": [{"product_id": 203, "quantity": 2}]}`
	req = httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	createOrderHandler(w, req)

	// ステータスコードがPaymentRequired (402)であることを確認
	if w.Code != http.StatusPaymentRequired {
		t.Errorf("Expected status %d for payment failure, got %d", http.StatusPaymentRequired, w.Code)
	}

	// 在庫が減っていないことを確認
	stockMux.RLock()
	if stocks["203-1"].Quantity != initialStock {
		t.Errorf("Stock should not be decreased on payment failure. Expected %d, got %d",
			initialStock, stocks["203-1"].Quantity)
	}
	stockMux.RUnlock()

	// 注文がpayment_failedステータスで保存されていることを確認
	orderMux.RLock()
	var failedOrder *Order
	for _, o := range orders {
		if o.UserID == testUser.ID && o.Status == "payment_failed" {
			failedOrder = o
			break
		}
	}
	orderMux.RUnlock()

	if failedOrder == nil {
		t.Error("Failed order should be saved with payment_failed status")
	}
}

func TestGetOrdersHandler(t *testing.T) {
	// テスト用ユーザーとトークンを設定
	testUser := &User{ID: 4, Username: "orderlistuser", IsAdmin: false}
	userToken := "orderlist-test-token"
	sessionMux.Lock()
	sessions[userToken] = testUser
	sessionMux.Unlock()

	// テスト用の注文を追加
	orderMux.Lock()
	orders[1000] = &Order{ID: 1000, UserID: 4, TotalPrice: 10000, Status: "confirmed"}
	orders[1001] = &Order{ID: 1001, UserID: 4, TotalPrice: 20000, Status: "confirmed"}
	orders[1002] = &Order{ID: 1002, UserID: 999, TotalPrice: 30000, Status: "confirmed"} // 他のユーザーの注文
	orderMux.Unlock()

	// 注文一覧取得
	req := httptest.NewRequest("GET", "/orders", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w := httptest.NewRecorder()
	getOrdersHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var userOrders []Order
	json.NewDecoder(w.Body).Decode(&userOrders)

	// ユーザー自身の注文のみが返されることを確認
	for _, order := range userOrders {
		if order.UserID != 4 {
			t.Errorf("Got order for different user: %d", order.UserID)
		}
	}

	// 認証なしでの注文一覧取得
	req = httptest.NewRequest("GET", "/orders", nil)
	w = httptest.NewRecorder()
	getOrdersHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d for no auth, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestGenerateToken(t *testing.T) {
	token1 := generateToken()
	token2 := generateToken()

	if token1 == "" {
		t.Error("Generated empty token")
	}

	if token1 == token2 {
		t.Error("Generated duplicate tokens")
	}
}

func TestGetAuthUser(t *testing.T) {
	// テストユーザーとトークンを設定
	testUser := &User{ID: 5, Username: "authtest", IsAdmin: false}
	testToken := "auth-test-token"
	sessionMux.Lock()
	sessions[testToken] = testUser
	sessionMux.Unlock()

	// 正常な認証
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	user := getAuthUser(req)

	if user == nil {
		t.Error("Expected user, got nil")
	} else if user.ID != 5 {
		t.Errorf("Expected user ID 5, got %d", user.ID)
	}

	// トークンなし
	req = httptest.NewRequest("GET", "/", nil)
	user = getAuthUser(req)

	if user != nil {
		t.Error("Expected nil for no token")
	}

	// 無効なトークン
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	user = getAuthUser(req)

	if user != nil {
		t.Error("Expected nil for invalid token")
	}
}

func TestCouponDiscount(t *testing.T) {
	// 固定額クーポンのテスト
	t.Run("FixedAmountCoupon", func(t *testing.T) {
		coupon := &Coupon{
			Code:   "FLAT1000",
			Type:   "fixed",
			Amount: 1000,
		}

		// 通常の計算
		discount := calculateCouponDiscount(coupon, 5000)
		if discount != 1000 {
			t.Errorf("Expected discount 1000, got %d", discount)
		}

		// 割引額が商品代金を超える場合
		discount = calculateCouponDiscount(coupon, 500)
		if discount != 500 {
			t.Errorf("Expected discount 500 (capped at base amount), got %d", discount)
		}
	})

	// パーセンテージクーポンのテスト
	t.Run("PercentageCoupon", func(t *testing.T) {
		coupon := &Coupon{
			Code:   "SAVE10",
			Type:   "percentage",
			Amount: 10,
		}

		discount := calculateCouponDiscount(coupon, 10000)
		if discount != 1000 {
			t.Errorf("Expected discount 1000 (10%% of 10000), got %d", discount)
		}

		coupon20 := &Coupon{
			Code:   "SAVE20",
			Type:   "percentage",
			Amount: 20,
		}

		discount = calculateCouponDiscount(coupon20, 5000)
		if discount != 1000 {
			t.Errorf("Expected discount 1000 (20%% of 5000), got %d", discount)
		}
	})

	// nilクーポンのテスト
	t.Run("NilCoupon", func(t *testing.T) {
		discount := calculateCouponDiscount(nil, 10000)
		if discount != 0 {
			t.Errorf("Expected discount 0 for nil coupon, got %d", discount)
		}
	})

	// 無効なクーポンタイプのテスト
	t.Run("InvalidCouponType", func(t *testing.T) {
		coupon := &Coupon{
			Code:   "INVALID",
			Type:   "invalid_type",
			Amount: 1000,
		}

		discount := calculateCouponDiscount(coupon, 5000)
		if discount != 0 {
			t.Errorf("Expected discount 0 for invalid coupon type, got %d", discount)
		}
	})
}

func TestCreateOrderWithCoupon(t *testing.T) {
	// 元の決済ゲートウェイを保存して後で復元
	originalGateway := paymentGateway
	defer func() { paymentGateway = originalGateway }()

	// 決済成功のモックを設定
	paymentGateway = &MockPaymentGateway{shouldSucceed: true}

	// テスト用ユーザーとトークンを設定
	testUser := &User{ID: 10, Username: "couponuser", IsAdmin: false}
	userToken := "coupon-test-token"
	sessionMux.Lock()
	sessions[userToken] = testUser
	sessionMux.Unlock()

	// テスト用商品を追加
	productMux.Lock()
	products[300] = &Product{ID: 300, Name: "テスト商品A", Price: 3000, Category: "テスト"}
	products[301] = &Product{ID: 301, Name: "テスト商品B", Price: 10000, Category: "テスト"}
	productMux.Unlock()

	// テスト用在庫を追加
	stockMux.Lock()
	stocks["300-1"] = &Stock{ProductID: 300, WarehouseID: 1, Quantity: 20}
	stocks["301-1"] = &Stock{ProductID: 301, WarehouseID: 1, Quantity: 10}
	stockMux.Unlock()

	// 固定額クーポンで送料無料のケース
	t.Run("FixedCouponWithFreeShipping", func(t *testing.T) {
		reqBody := `{"items": [{"product_id": 301, "quantity": 1}], "coupon_code": "FLAT1000"}`
		req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		createOrderHandler(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
		}

		var order Order
		json.NewDecoder(w.Body).Decode(&order)
		// 商品: 10000円, 消費税: 1000円, 小計: 11000円
		// 送料: 0円（5000円以上）
		// クーポン割引: 1000円（固定額）
		// 合計: 11000 - 1000 + 0 = 10000円
		expectedTotal := 10000
		if order.TotalPrice != expectedTotal {
			t.Errorf("Expected total %d with FLAT1000 coupon, got %d", expectedTotal, order.TotalPrice)
		}
		if order.DiscountAmount != 1000 {
			t.Errorf("Expected discount 1000, got %d", order.DiscountAmount)
		}
		if order.AppliedCoupon != "FLAT1000" {
			t.Errorf("Expected applied coupon FLAT1000, got %s", order.AppliedCoupon)
		}
		if order.ShippingFee != 0 {
			t.Errorf("Expected shipping fee 0, got %d", order.ShippingFee)
		}
	})

	// パーセンテージクーポンで送料ありのケース
	t.Run("PercentageCouponWithShipping", func(t *testing.T) {
		reqBody := `{"items": [{"product_id": 300, "quantity": 1}], "coupon_code": "SAVE20"}`
		req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		createOrderHandler(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
		}

		var order Order
		json.NewDecoder(w.Body).Decode(&order)
		// 商品: 3000円, 消費税: 300円, 小計: 3300円
		// 送料: 500円（5000円未満）
		// クーポン割引: 660円（3300円の20%）
		// 合計: 3300 - 660 + 500 = 3140円
		expectedTotal := 3140
		if order.TotalPrice != expectedTotal {
			t.Errorf("Expected total %d with SAVE20 coupon, got %d", expectedTotal, order.TotalPrice)
		}
		if order.DiscountAmount != 660 {
			t.Errorf("Expected discount 660 (20%% of 3300), got %d", order.DiscountAmount)
		}
		if order.AppliedCoupon != "SAVE20" {
			t.Errorf("Expected applied coupon SAVE20, got %s", order.AppliedCoupon)
		}
		if order.ShippingFee != 500 {
			t.Errorf("Expected shipping fee 500, got %d", order.ShippingFee)
		}
	})

	// 無効なクーポンコードのテスト
	t.Run("InvalidCouponCode", func(t *testing.T) {
		reqBody := `{"items": [{"product_id": 300, "quantity": 1}], "coupon_code": "INVALID_CODE"}`
		req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		createOrderHandler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d for invalid coupon, got %d", http.StatusBadRequest, w.Code)
		}
	})

	// クーポンなしの注文（既存機能の確認）
	t.Run("OrderWithoutCoupon", func(t *testing.T) {
		reqBody := `{"items": [{"product_id": 300, "quantity": 2}]}`
		req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		createOrderHandler(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
		}

		var order Order
		json.NewDecoder(w.Body).Decode(&order)
		// 商品: 6000円, 消費税: 600円, 小計: 6600円
		// 送料: 0円（5000円以上）
		// クーポン割引: 0円
		// 合計: 6600円
		expectedTotal := 6600
		if order.TotalPrice != expectedTotal {
			t.Errorf("Expected total %d without coupon, got %d", expectedTotal, order.TotalPrice)
		}
		if order.DiscountAmount != 0 {
			t.Errorf("Expected discount 0, got %d", order.DiscountAmount)
		}
		if order.AppliedCoupon != "" {
			t.Errorf("Expected no applied coupon, got %s", order.AppliedCoupon)
		}
	})

	// 複数商品でのクーポン適用テスト
	t.Run("MultipleItemsWithCoupon", func(t *testing.T) {
		reqBody := `{"items": [{"product_id": 300, "quantity": 2}, {"product_id": 301, "quantity": 1}], "coupon_code": "FLAT2000"}`
		req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		createOrderHandler(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
		}

		var order Order
		json.NewDecoder(w.Body).Decode(&order)
		// 商品: 3000*2 + 10000 = 16000円, 消費税: 1600円, 小計: 17600円
		// 送料: 0円（5000円以上）
		// クーポン割引: 2000円（固定額）
		// 合計: 17600 - 2000 + 0 = 15600円
		expectedTotal := 15600
		if order.TotalPrice != expectedTotal {
			t.Errorf("Expected total %d with FLAT2000 coupon, got %d", expectedTotal, order.TotalPrice)
		}
		if order.DiscountAmount != 2000 {
			t.Errorf("Expected discount 2000, got %d", order.DiscountAmount)
		}
	})

	// 決済失敗時のクーポン処理テスト
	t.Run("PaymentFailureWithCoupon", func(t *testing.T) {
		// 決済失敗のモックを設定
		paymentGateway = &MockPaymentGateway{shouldSucceed: false}

		reqBody := `{"items": [{"product_id": 300, "quantity": 1}], "coupon_code": "SAVE10"}`
		req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		createOrderHandler(w, req)

		if w.Code != http.StatusPaymentRequired {
			t.Errorf("Expected status %d for payment failure, got %d", http.StatusPaymentRequired, w.Code)
		}

		// 注文が失敗ステータスで保存されているか確認
		orderMux.RLock()
		var failedOrder *Order
		for _, o := range orders {
			if o.UserID == testUser.ID && o.Status == "payment_failed" && o.AppliedCoupon == "SAVE10" {
				failedOrder = o
				break
			}
		}
		orderMux.RUnlock()

		if failedOrder == nil {
			t.Error("Failed order with coupon should be saved with payment_failed status")
		} else {
			// クーポン情報が保存されているか確認
			if failedOrder.AppliedCoupon != "SAVE10" {
				t.Errorf("Expected applied coupon SAVE10 in failed order, got %s", failedOrder.AppliedCoupon)
			}
			if failedOrder.DiscountAmount != 330 { // 3300円の10%
				t.Errorf("Expected discount amount 330 in failed order, got %d", failedOrder.DiscountAmount)
			}
		}
	})
}

func TestSalesReportHandler(t *testing.T) {
	// 元の決済ゲートウェイを保存して後で復元
	originalGateway := paymentGateway
	defer func() { paymentGateway = originalGateway }()

	// 決済成功のモックを設定
	paymentGateway = &MockPaymentGateway{shouldSucceed: true}

	// 管理者トークンを設定
	adminUser := &User{ID: 1, Username: "admin", IsAdmin: true}
	adminToken := "admin-report-token"
	sessionMux.Lock()
	sessions[adminToken] = adminUser
	sessionMux.Unlock()

	// 一般ユーザートークンを設定
	regularUser := &User{ID: 11, Username: "regularuser", IsAdmin: false}
	regularToken := "regular-report-token"
	sessionMux.Lock()
	sessions[regularToken] = regularUser
	sessionMux.Unlock()

	// テストデータの準備 - 商品を追加
	productMux.Lock()
	products[400] = &Product{ID: 400, Name: "レポート商品A", Price: 5000, Category: "テスト"}
	products[401] = &Product{ID: 401, Name: "レポート商品B", Price: 10000, Category: "テスト"}
	products[402] = &Product{ID: 402, Name: "レポート商品C", Price: 3000, Category: "テスト"}
	productMux.Unlock()

	// 在庫を追加（既存の在庫を一時的に退避）
	stockMux.Lock()
	originalStocks := stocks
	stocks = make(map[string]*Stock)
	stocks["400-1"] = &Stock{ProductID: 400, WarehouseID: 1, Quantity: 10}
	stocks["400-2"] = &Stock{ProductID: 400, WarehouseID: 2, Quantity: 5}
	stocks["401-1"] = &Stock{ProductID: 401, WarehouseID: 1, Quantity: 8}
	stocks["402-3"] = &Stock{ProductID: 402, WarehouseID: 3, Quantity: 12}
	stockMux.Unlock()

	// テスト後に在庫を復元
	defer func() {
		stockMux.Lock()
		stocks = originalStocks
		stockMux.Unlock()
	}()

	// テスト用の注文を作成（既存の注文も一時的に退避）
	orderMux.Lock()
	originalOrders := orders
	orders = make(map[int]*Order)
	// テスト後に注文を復元
	defer func() {
		orderMux.Lock()
		orders = originalOrders
		orderMux.Unlock()
	}()

	// 完了した注文（集計対象）
	orders[2000] = &Order{
		ID:         2000,
		UserID:     11,
		Items:      []OrderItem{{ProductID: 400, Quantity: 3}, {ProductID: 401, Quantity: 2}},
		TotalPrice: 25000,
		Status:     "completed",
		AppliedCoupon: "SAVE10",
	}
	orders[2001] = &Order{
		ID:         2001,
		UserID:     11,
		Items:      []OrderItem{{ProductID: 401, Quantity: 1}},
		TotalPrice: 11000,
		Status:     "completed",
	}
	orders[2002] = &Order{
		ID:         2002,
		UserID:     11,
		Items:      []OrderItem{{ProductID: 402, Quantity: 5}},
		TotalPrice: 16500,
		Status:     "completed",
		AppliedCoupon: "FLAT1000",
	}
	// 失敗した注文（集計から除外）
	orders[2003] = &Order{
		ID:         2003,
		UserID:     11,
		Items:      []OrderItem{{ProductID: 400, Quantity: 10}},
		TotalPrice: 55000,
		Status:     "payment_failed",
		AppliedCoupon: "SAVE20",
	}
	orderMux.Unlock()

	// 管理者によるレポート取得のテスト
	t.Run("AdminAccessReport", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/reports/sales", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()
		getSalesReportHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var report SalesReportResponse
		json.NewDecoder(w.Body).Decode(&report)

		// 売上サマリーの検証
		expectedRevenue := 25000 + 11000 + 16500 // 52500
		if report.SalesSummary.TotalRevenue != expectedRevenue {
			t.Errorf("Expected total revenue %d, got %d", expectedRevenue, report.SalesSummary.TotalRevenue)
		}
		if report.SalesSummary.TotalOrders != 3 {
			t.Errorf("Expected 3 completed orders, got %d", report.SalesSummary.TotalOrders)
		}

		// 人気商品ランキングの検証（商品402が5個、商品401が3個、商品400が3個の順）
		if len(report.TopProducts) > 0 {
			if report.TopProducts[0].ProductName != "レポート商品C" || report.TopProducts[0].TotalQuantity != 5 {
				t.Errorf("Expected top product to be レポート商品C with 5 sales, got %s with %d",
					report.TopProducts[0].ProductName, report.TopProducts[0].TotalQuantity)
			}
		}

		// 倉庫別在庫の検証
		foundTokyo := false
		for _, warehouse := range report.WarehouseInventory {
			if warehouse.WarehouseName == "東京倉庫" {
				foundTokyo = true
				expectedStock := 10 + 8 // 商品400と401の在庫
				if warehouse.TotalStock != expectedStock {
					t.Errorf("Expected Tokyo warehouse stock %d, got %d", expectedStock, warehouse.TotalStock)
				}
				break
			}
		}
		if !foundTokyo {
			t.Error("Tokyo warehouse not found in inventory report")
		}

		// クーポン利用率の検証（4注文中3つでクーポン使用 = 75%）
		expectedRate := 75.0
		if report.PromotionAnalysis.CouponUsageRate != expectedRate {
			t.Errorf("Expected coupon usage rate %.1f%%, got %.1f%%",
				expectedRate, report.PromotionAnalysis.CouponUsageRate)
		}
	})

	// 一般ユーザーによるアクセス拒否のテスト
	t.Run("RegularUserAccessDenied", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/reports/sales", nil)
		req.Header.Set("Authorization", "Bearer "+regularToken)
		w := httptest.NewRecorder()
		getSalesReportHandler(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status %d for non-admin, got %d", http.StatusForbidden, w.Code)
		}
	})

	// 認証なしでのアクセス拒否のテスト
	t.Run("UnauthenticatedAccessDenied", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/reports/sales", nil)
		w := httptest.NewRecorder()
		getSalesReportHandler(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d for unauthenticated, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	// データがない場合のテスト
	t.Run("EmptyDataReport", func(t *testing.T) {
		// 既存の注文を一時的に退避
		orderMux.Lock()
		tempOrders := orders
		orders = make(map[int]*Order)
		orderMux.Unlock()

		req := httptest.NewRequest("GET", "/admin/reports/sales", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()
		getSalesReportHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d for empty data, got %d", http.StatusOK, w.Code)
		}

		var report SalesReportResponse
		json.NewDecoder(w.Body).Decode(&report)

		if report.SalesSummary.TotalRevenue != 0 {
			t.Errorf("Expected 0 revenue for empty data, got %d", report.SalesSummary.TotalRevenue)
		}
		if report.SalesSummary.TotalOrders != 0 {
			t.Errorf("Expected 0 orders for empty data, got %d", report.SalesSummary.TotalOrders)
		}
		if len(report.TopProducts) != 0 {
			t.Errorf("Expected empty top products, got %d", len(report.TopProducts))
		}

		// データを復元
		orderMux.Lock()
		orders = tempOrders
		orderMux.Unlock()
	})
}

func TestWishlistOperations(t *testing.T) {
	// テスト用ユーザーとトークンを設定
	testUser := &User{ID: 50, Username: "wishlistuser", IsAdmin: false}
	userToken := "wishlist-test-token"
	sessionMux.Lock()
	sessions[userToken] = testUser
	sessionMux.Unlock()

	// テスト用商品を追加（専用カテゴリを使用）
	productMux.Lock()
	products[500] = &Product{ID: 500, Name: "ウィッシュリスト商品A", Price: 5000, Category: "ウィッシュリストテスト"}
	products[501] = &Product{ID: 501, Name: "ウィッシュリスト商品B", Price: 10000, Category: "ウィッシュリストテスト"}
	products[502] = &Product{ID: 502, Name: "ウィッシュリスト商品C", Price: 3000, Category: "別カテゴリ"}
	productMux.Unlock()

	// テスト用在庫を追加
	stockMux.Lock()
	stocks["500-1"] = &Stock{ProductID: 500, WarehouseID: 1, Quantity: 10}
	stocks["501-1"] = &Stock{ProductID: 501, WarehouseID: 1, Quantity: 5}
	stocks["502-1"] = &Stock{ProductID: 502, WarehouseID: 1, Quantity: 8}
	stockMux.Unlock()

	// お気に入り追加のテスト
	t.Run("AddToWishlist", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/wishlist/500", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		addToWishlistHandler(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(w.Body).Decode(&response)
		if response["message"] != "Added to wishlist" {
			t.Errorf("Expected message 'Added to wishlist', got %v", response["message"])
		}
		if int(response["product_id"].(float64)) != 500 {
			t.Errorf("Expected product_id 500, got %v", response["product_id"])
		}

		// 実際にウィッシュリストに追加されているか確認
		if !isProductInWishlist(testUser.ID, 500) {
			t.Error("Product should be in wishlist")
		}
	})

	// 重複追加のテスト
	t.Run("AddDuplicateToWishlist", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/wishlist/500", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		addToWishlistHandler(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("Expected status %d for duplicate, got %d", http.StatusConflict, w.Code)
		}
	})

	// 存在しない商品の追加テスト
	t.Run("AddNonexistentProductToWishlist", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/wishlist/999999", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		addToWishlistHandler(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status %d for nonexistent product, got %d", http.StatusNotFound, w.Code)
		}
	})

	// 認証なしでの追加テスト
	t.Run("AddToWishlistWithoutAuth", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/wishlist/501", nil)
		w := httptest.NewRecorder()
		addToWishlistHandler(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d for no auth, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	// 商品一覧でis_favoriteフィールドの確認
	t.Run("ProductListWithFavorite", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/products?category=ウィッシュリストテスト", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		getProductsHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var products []ProductDetailResponseWithFavorite
		json.NewDecoder(w.Body).Decode(&products)

		foundFavorite := false
		foundNonFavorite := false
		for _, p := range products {
			if p.ID == 500 && p.IsFavorite {
				foundFavorite = true
			}
			if p.ID == 501 && !p.IsFavorite {
				foundNonFavorite = true
			}
		}
		if !foundFavorite {
			t.Error("Product 500 should be marked as favorite")
		}
		if !foundNonFavorite {
			t.Error("Product 501 should not be marked as favorite")
		}
	})

	// 商品詳細でis_favoriteフィールドの確認
	t.Run("ProductDetailWithFavorite", func(t *testing.T) {
		// お気に入りに登録済みの商品
		req := httptest.NewRequest("GET", "/products/500", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		getProductHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var product ProductDetailResponseWithFavorite
		json.NewDecoder(w.Body).Decode(&product)
		if !product.IsFavorite {
			t.Error("Product 500 should be marked as favorite")
		}

		// お気に入りに未登録の商品
		req = httptest.NewRequest("GET", "/products/501", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w = httptest.NewRecorder()
		getProductHandler(w, req)

		json.NewDecoder(w.Body).Decode(&product)
		if product.IsFavorite {
			t.Error("Product 501 should not be marked as favorite")
		}
	})

	// おすすめ商品取得（お気に入りに追加してから）
	t.Run("GetRecommendations", func(t *testing.T) {
		// まず商品500をお気に入りに追加
		req := httptest.NewRequest("POST", "/wishlist/500", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		addToWishlistHandler(w, req)

		if w.Code != http.StatusCreated && w.Code != http.StatusConflict {
			t.Fatalf("Failed to add product to wishlist: status %d", w.Code)
		}

		// デバッグ：現在のwishlistの状態を確認
		wishlistMux.RLock()
		wishlistCount := 0
		for key, wishlist := range wishlists {
			if wishlist != nil && wishlist.UserID == testUser.ID {
				t.Logf("Wishlist item found: key=%s, UserID=%d, ProductID=%d", key, wishlist.UserID, wishlist.ProductID)
				wishlistCount++
			}
		}
		t.Logf("Total wishlist items for user %d: %d", testUser.ID, wishlistCount)
		wishlistMux.RUnlock()

		// 同じカテゴリの商品501もお気に入りにない状態でおすすめされるはず
		req = httptest.NewRequest("GET", "/users/me/recommendations", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w = httptest.NewRecorder()
		getRecommendationsHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var recommendations []RecommendedProduct
		json.NewDecoder(w.Body).Decode(&recommendations)

		// デバッグ情報を出力
		t.Logf("Number of recommendations: %d", len(recommendations))
		for i, r := range recommendations {
			t.Logf("Recommendation %d: ID=%d, Name=%s, Category=%s", i, r.ID, r.Name, r.Category)
		}

		foundRecommendation := false
		for _, r := range recommendations {
			if r.ID == 501 && r.Category == "ウィッシュリストテスト" {
				foundRecommendation = true
				break
			}
		}
		if !foundRecommendation {
			t.Error("Product 501 should be recommended (same category as favorite)")
		}

		// 別カテゴリの商品502は推薦されないはず
		for _, r := range recommendations {
			if r.ID == 502 {
				t.Error("Product 502 should not be recommended (different category)")
			}
		}
	})

	// おすすめ商品取得（認証なし）
	t.Run("GetRecommendationsWithoutAuth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/me/recommendations", nil)
		w := httptest.NewRecorder()
		getRecommendationsHandler(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d for no auth, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	// お気に入り削除のテスト
	t.Run("RemoveFromWishlist", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/wishlist/500", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		removeFromWishlistHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(w.Body).Decode(&response)
		if response["message"] != "Removed from wishlist" {
			t.Errorf("Expected message 'Removed from wishlist', got %v", response["message"])
		}

		// 実際にウィッシュリストから削除されているか確認
		if isProductInWishlist(testUser.ID, 500) {
			t.Error("Product should not be in wishlist")
		}
	})

	// 未登録商品の削除テスト
	t.Run("RemoveNonexistentFromWishlist", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/wishlist/501", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		removeFromWishlistHandler(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status %d for not in wishlist, got %d", http.StatusNotFound, w.Code)
		}
	})

	// 削除（認証なし）
	t.Run("RemoveFromWishlistWithoutAuth", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/wishlist/500", nil)
		w := httptest.NewRecorder()
		removeFromWishlistHandler(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d for no auth, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	// 不正なHTTPメソッドのテスト
	t.Run("InvalidMethodForWishlist", func(t *testing.T) {
		// GETメソッドでお気に入り追加を試みる
		req := httptest.NewRequest("GET", "/wishlist/500", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		addToWishlistHandler(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status %d for invalid method, got %d", http.StatusMethodNotAllowed, w.Code)
		}

		// PUTメソッドでお気に入り削除を試みる
		req = httptest.NewRequest("PUT", "/wishlist/500", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w = httptest.NewRecorder()
		removeFromWishlistHandler(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status %d for invalid method, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})

	// クリーンアップ（テスト用のウィッシュリストデータを削除）
	wishlistMux.Lock()
	for key := range wishlists {
		if strings.HasPrefix(key, "50-") {
			delete(wishlists, key)
		}
	}
	wishlistMux.Unlock()
}

func TestMainHandler(t *testing.T) {
	// 404のテスト
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	mainHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d for nonexistent route, got %d", http.StatusNotFound, w.Code)
	}

	// メソッド不一致のテスト（サポートされていないメソッド）
	// 注: mainHandlerのルーティングでは未サポートのメソッドも404を返す実装になっている
	req = httptest.NewRequest("DELETE", "/products", nil)
	w = httptest.NewRecorder()
	mainHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d for unsupported method, got %d", http.StatusNotFound, w.Code)
	}
}