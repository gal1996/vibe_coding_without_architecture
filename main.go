package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// エンティティ定義
type Product struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Price    int    `json:"price"`
	Category string `json:"category"`
}

// 倉庫エンティティ
type Warehouse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// 在庫エンティティ（商品と倉庫の関連）
type Stock struct {
	ProductID   int `json:"product_id"`
	WarehouseID int `json:"warehouse_id"`
	Quantity    int `json:"quantity"`
}

// 商品詳細レスポンス用の構造体
type ProductDetailResponse struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	Price       int              `json:"price"`
	Category    string           `json:"category"`
	TotalStock  int              `json:"total_stock"`
	StockDetail []StockWarehouse `json:"stock_detail"`
}

type StockWarehouse struct {
	WarehouseName string `json:"warehouse_name"`
	Quantity      int    `json:"quantity"`
}

type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	IsAdmin      bool   `json:"is_admin"`
	Token        string `json:"token,omitempty"`
}

type OrderItem struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

type Order struct {
	ID             int         `json:"id"`
	UserID         int         `json:"user_id"`
	Items          []OrderItem `json:"items"`
	TotalPrice     int         `json:"total_price"`
	ShippingFee    int         `json:"shipping_fee"`
	DiscountAmount int         `json:"discount_amount"`
	AppliedCoupon  string      `json:"applied_coupon,omitempty"`
	Status         string      `json:"status"`
	CreatedAt      time.Time   `json:"created_at"`
}

// クーポンエンティティ
type Coupon struct {
	Code         string `json:"code"`
	Type         string `json:"type"`         // "fixed" or "percentage"
	Amount       int    `json:"amount"`       // 固定額または割合（%）
	Description  string `json:"description"`
}

// 決済関連の型定義
type PaymentResult struct {
	Success       bool   `json:"success"`
	TransactionID string `json:"transaction_id"`
	Message       string `json:"message"`
}

type PaymentGateway interface {
	ProcessPayment(amount int, orderID int) PaymentResult
}

// データストア（インメモリ）
var (
	products    = make(map[int]*Product)
	warehouses  = make(map[int]*Warehouse)
	stocks      = make(map[string]*Stock) // key: "productID-warehouseID"
	users       = make(map[int]*User)
	usersByName = make(map[string]*User)
	orders      = make(map[int]*Order)
	sessions    = make(map[string]*User)
	coupons     = make(map[string]*Coupon)

	productMux   sync.RWMutex
	warehouseMux sync.RWMutex
	stockMux     sync.RWMutex
	userMux      sync.RWMutex
	orderMux     sync.RWMutex
	sessionMux   sync.RWMutex
	couponMux    sync.RWMutex

	nextProductID   = 1
	nextWarehouseID = 1
	nextUserID      = 1
	nextOrderID     = 1
)

// ダミー決済ゲートウェイの実装
type DummyPaymentGateway struct{}

func (d *DummyPaymentGateway) ProcessPayment(amount int, orderID int) PaymentResult {
	// 90%の確率で成功するようにシミュレート
	rand.Seed(time.Now().UnixNano())
	success := rand.Float64() < 0.9

	if success {
		// 成功時はトランザクションIDを生成
		transactionID := fmt.Sprintf("TXN_%d_%d", time.Now().Unix(), orderID)
		return PaymentResult{
			Success:       true,
			TransactionID: transactionID,
			Message:       "Payment processed successfully",
		}
	}

	// 失敗時のメッセージ
	failureReasons := []string{
		"Insufficient funds",
		"Card declined",
		"Payment gateway timeout",
		"Invalid card details",
	}
	reason := failureReasons[rand.Intn(len(failureReasons))]

	return PaymentResult{
		Success:       false,
		TransactionID: "",
		Message:       reason,
	}
}

// グローバルな決済ゲートウェイインスタンス
var paymentGateway PaymentGateway = &DummyPaymentGateway{}

// 初期データ
func init() {
	// 管理者ユーザーを作成
	adminPass, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	admin := &User{
		ID:           nextUserID,
		Username:     "admin",
		PasswordHash: string(adminPass),
		IsAdmin:      true,
	}
	users[admin.ID] = admin
	usersByName[admin.Username] = admin
	nextUserID++

	// 倉庫を作成
	warehouses[1] = &Warehouse{ID: 1, Name: "東京倉庫"}
	warehouses[2] = &Warehouse{ID: 2, Name: "大阪倉庫"}
	warehouses[3] = &Warehouse{ID: 3, Name: "福岡倉庫"}
	nextWarehouseID = 4

	// サンプル商品を追加
	sampleProducts := []Product{
		{ID: 1, Name: "ノートPC", Price: 120000, Category: "電子機器"},
		{ID: 2, Name: "マウス", Price: 3000, Category: "電子機器"},
		{ID: 3, Name: "デスク", Price: 25000, Category: "家具"},
		{ID: 4, Name: "チェア", Price: 15000, Category: "家具"},
	}

	for i := range sampleProducts {
		product := sampleProducts[i]
		products[product.ID] = &product
	}
	nextProductID = 5

	// 各商品の倉庫別在庫を設定
	// ノートPC: 東京5、大阪3、福岡2 (合計10)
	stocks["1-1"] = &Stock{ProductID: 1, WarehouseID: 1, Quantity: 5}
	stocks["1-2"] = &Stock{ProductID: 1, WarehouseID: 2, Quantity: 3}
	stocks["1-3"] = &Stock{ProductID: 1, WarehouseID: 3, Quantity: 2}

	// マウス: 東京20、大阪20、福岡10 (合計50)
	stocks["2-1"] = &Stock{ProductID: 2, WarehouseID: 1, Quantity: 20}
	stocks["2-2"] = &Stock{ProductID: 2, WarehouseID: 2, Quantity: 20}
	stocks["2-3"] = &Stock{ProductID: 2, WarehouseID: 3, Quantity: 10}

	// デスク: 東京2、大阪2、福岡1 (合計5)
	stocks["3-1"] = &Stock{ProductID: 3, WarehouseID: 1, Quantity: 2}
	stocks["3-2"] = &Stock{ProductID: 3, WarehouseID: 2, Quantity: 2}
	stocks["3-3"] = &Stock{ProductID: 3, WarehouseID: 3, Quantity: 1}

	// チェア: 東京3、大阪3、福岡2 (合計8)
	stocks["4-1"] = &Stock{ProductID: 4, WarehouseID: 1, Quantity: 3}
	stocks["4-2"] = &Stock{ProductID: 4, WarehouseID: 2, Quantity: 3}
	stocks["4-3"] = &Stock{ProductID: 4, WarehouseID: 3, Quantity: 2}

	// クーポンマスターデータを作成
	coupons["SAVE10"] = &Coupon{
		Code:        "SAVE10",
		Type:        "percentage",
		Amount:      10,
		Description: "10%割引クーポン",
	}
	coupons["SAVE20"] = &Coupon{
		Code:        "SAVE20",
		Type:        "percentage",
		Amount:      20,
		Description: "20%割引クーポン",
	}
	coupons["FLAT1000"] = &Coupon{
		Code:        "FLAT1000",
		Type:        "fixed",
		Amount:      1000,
		Description: "1000円割引クーポン",
	}
	coupons["FLAT2000"] = &Coupon{
		Code:        "FLAT2000",
		Type:        "fixed",
		Amount:      2000,
		Description: "2000円割引クーポン",
	}
}

// ユーティリティ関数
var tokenCounter int

func generateToken() string {
	tokenCounter++
	return fmt.Sprintf("token_%d_%d_%d", time.Now().UnixNano(), nextUserID, tokenCounter)
}

func getAuthUser(r *http.Request) *User {
	token := r.Header.Get("Authorization")
	if token == "" {
		return nil
	}

	token = strings.TrimPrefix(token, "Bearer ")
	sessionMux.RLock()
	user := sessions[token]
	sessionMux.RUnlock()

	return user
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

// 在庫管理ヘルパー関数
func getProductStock(productID int) (totalStock int, stockDetails []StockWarehouse) {
	stockMux.RLock()
	defer stockMux.RUnlock()

	for _, stock := range stocks {
		if stock.ProductID == productID && stock.Quantity > 0 {
			warehouseMux.RLock()
			warehouse := warehouses[stock.WarehouseID]
			warehouseMux.RUnlock()

			if warehouse != nil {
				stockDetails = append(stockDetails, StockWarehouse{
					WarehouseName: warehouse.Name,
					Quantity:      stock.Quantity,
				})
				totalStock += stock.Quantity
			}
		}
	}
	return
}

// 在庫を引き当てる関数
func allocateStock(productID int, requiredQuantity int) (allocated bool, allocations map[int]int) {
	allocations = make(map[int]int)
	remaining := requiredQuantity

	// まず在庫を確認
	stockMux.RLock()
	var availableStocks []*Stock
	for _, stock := range stocks {
		if stock.ProductID == productID && stock.Quantity > 0 {
			availableStocks = append(availableStocks, stock)
		}
	}
	stockMux.RUnlock()

	// 在庫が存在する倉庫から順に引き当て
	for _, stock := range availableStocks {
		if remaining <= 0 {
			break
		}

		if stock.Quantity >= remaining {
			allocations[stock.WarehouseID] = remaining
			remaining = 0
		} else {
			allocations[stock.WarehouseID] = stock.Quantity
			remaining -= stock.Quantity
		}
	}

	// 全数量を確保できた場合のみ実際に在庫を減らす
	if remaining == 0 {
		stockMux.Lock()
		for warehouseID, quantity := range allocations {
			key := fmt.Sprintf("%d-%d", productID, warehouseID)
			if stock, exists := stocks[key]; exists {
				stock.Quantity -= quantity
			}
		}
		stockMux.Unlock()
		return true, allocations
	}

	return false, nil
}

// クーポン割引計算ヘルパー関数
func calculateCouponDiscount(coupon *Coupon, baseAmount int) int {
	if coupon == nil {
		return 0
	}

	var discount int
	switch coupon.Type {
	case "fixed":
		discount = coupon.Amount
	case "percentage":
		discount = baseAmount * coupon.Amount / 100
	default:
		return 0
	}

	// 割引額が元の金額を超えないようにする
	if discount > baseAmount {
		discount = baseAmount
	}

	return discount
}

// ハンドラー関数

// 商品一覧取得（カテゴリフィルタ対応）
func getProductsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	category := r.URL.Query().Get("category")

	productMux.RLock()
	defer productMux.RUnlock()

	var result []ProductDetailResponse
	for _, p := range products {
		if category == "" || p.Category == category {
			totalStock, stockDetails := getProductStock(p.ID)
			result = append(result, ProductDetailResponse{
				ID:          p.ID,
				Name:        p.Name,
				Price:       p.Price,
				Category:    p.Category,
				TotalStock:  totalStock,
				StockDetail: stockDetails,
			})
		}
	}

	jsonResponse(w, http.StatusOK, result)
}

// 商品詳細取得
func getProductHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// URLから商品IDを取得
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		errorResponse(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	id, err := strconv.Atoi(parts[2])
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	productMux.RLock()
	product := products[id]
	productMux.RUnlock()

	if product == nil {
		errorResponse(w, http.StatusNotFound, "Product not found")
		return
	}

	// 倉庫別在庫情報を取得
	totalStock, stockDetails := getProductStock(product.ID)

	response := ProductDetailResponse{
		ID:          product.ID,
		Name:        product.Name,
		Price:       product.Price,
		Category:    product.Category,
		TotalStock:  totalStock,
		StockDetail: stockDetails,
	}

	jsonResponse(w, http.StatusOK, response)
}

// 商品作成（管理者のみ）
func createProductHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 認証確認
	user := getAuthUser(r)
	if user == nil {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// 管理者権限確認
	if !user.IsAdmin {
		errorResponse(w, http.StatusForbidden, "Admin access required")
		return
	}

	var req struct {
		Name         string `json:"name"`
		Price        int    `json:"price"`
		Category     string `json:"category"`
		InitialStock int    `json:"initial_stock"` // 初期在庫（東京倉庫に配置）
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// バリデーション
	if req.Name == "" || req.Price <= 0 || req.InitialStock < 0 || req.Category == "" {
		errorResponse(w, http.StatusBadRequest, "Invalid product data")
		return
	}

	productMux.Lock()
	product := Product{
		ID:       nextProductID,
		Name:     req.Name,
		Price:    req.Price,
		Category: req.Category,
	}
	nextProductID++
	products[product.ID] = &product
	productMux.Unlock()

	// 初期在庫を東京倉庫（ID=1）に設定
	if req.InitialStock > 0 {
		stockMux.Lock()
		key := fmt.Sprintf("%d-%d", product.ID, 1)
		stocks[key] = &Stock{
			ProductID:   product.ID,
			WarehouseID: 1,
			Quantity:    req.InitialStock,
		}
		stockMux.Unlock()
	}

	// レスポンス用に在庫情報を含める
	totalStock, stockDetails := getProductStock(product.ID)
	response := ProductDetailResponse{
		ID:          product.ID,
		Name:        product.Name,
		Price:       product.Price,
		Category:    product.Category,
		TotalStock:  totalStock,
		StockDetail: stockDetails,
	}

	jsonResponse(w, http.StatusCreated, response)
}

// ユーザー登録
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		errorResponse(w, http.StatusBadRequest, "Username and password required")
		return
	}

	userMux.Lock()
	defer userMux.Unlock()

	// ユーザー名の重複チェック
	if usersByName[req.Username] != nil {
		errorResponse(w, http.StatusConflict, "Username already exists")
		return
	}

	// パスワードハッシュ化
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	user := &User{
		ID:           nextUserID,
		Username:     req.Username,
		PasswordHash: string(hash),
		IsAdmin:      false,
	}

	nextUserID++
	users[user.ID] = user
	usersByName[user.Username] = user

	// トークン生成
	token := generateToken()
	sessionMux.Lock()
	sessions[token] = user
	sessionMux.Unlock()

	user.Token = token
	jsonResponse(w, http.StatusCreated, user)
}

// ログイン
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	userMux.RLock()
	user := usersByName[req.Username]
	userMux.RUnlock()

	if user == nil {
		errorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// パスワード検証
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		errorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// トークン生成
	token := generateToken()
	sessionMux.Lock()
	sessions[token] = user
	sessionMux.Unlock()

	response := *user
	response.Token = token
	jsonResponse(w, http.StatusOK, response)
}

// 注文作成
func createOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 認証確認
	user := getAuthUser(r)
	if user == nil {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Items      []OrderItem `json:"items"`
		CouponCode string      `json:"coupon_code,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Items) == 0 {
		errorResponse(w, http.StatusBadRequest, "No items in order")
		return
	}

	// クーポンコードのバリデーション
	var appliedCoupon *Coupon
	if req.CouponCode != "" {
		couponMux.RLock()
		appliedCoupon = coupons[req.CouponCode]
		couponMux.RUnlock()

		if appliedCoupon == nil {
			errorResponse(w, http.StatusBadRequest, "Invalid coupon code")
			return
		}
	}

	// 在庫チェックと合計金額計算（税抜）
	subtotal := 0
	orderProducts := make([]*Product, len(req.Items))
	stockAllocations := make(map[int]map[int]int) // productID -> warehouseID -> quantity

	// 商品の存在確認と価格計算
	productMux.RLock()
	for i, item := range req.Items {
		if item.Quantity <= 0 {
			productMux.RUnlock()
			errorResponse(w, http.StatusBadRequest, fmt.Sprintf("Invalid quantity for item %d", i))
			return
		}

		product := products[item.ProductID]
		if product == nil {
			productMux.RUnlock()
			errorResponse(w, http.StatusNotFound, fmt.Sprintf("Product %d not found", item.ProductID))
			return
		}

		// 総在庫数を確認
		totalStock, _ := getProductStock(product.ID)
		if totalStock < item.Quantity {
			productMux.RUnlock()
			errorResponse(w, http.StatusBadRequest,
				fmt.Sprintf("Insufficient stock for product %s (available: %d, requested: %d)",
					product.Name, totalStock, item.Quantity))
			return
		}

		orderProducts[i] = product
		subtotal += product.Price * item.Quantity
		stockAllocations[product.ID] = make(map[int]int)
	}
	productMux.RUnlock()

	// 支払い金額の算出アルゴリズム（仕様書の順序に従う）
	// 1. 商品小計（既に計算済み: subtotal）

	// 2. 消費税の加算（10%）
	tax := subtotal / 10
	subtotalWithTax := subtotal + tax

	// 3. 送料の確定（消費税加算後の金額に基づく）
	shippingFee := 0
	if subtotalWithTax < 5000 {
		shippingFee = 500
	}

	// 4. クーポン割引の適用（商品代金＋消費税に対して、送料は対象外）
	discountAmount := calculateCouponDiscount(appliedCoupon, subtotalWithTax)
	discountedAmount := subtotalWithTax - discountAmount

	// 5. 最終金額の確定（割引後金額＋送料）
	totalPrice := discountedAmount + shippingFee

	// 注文IDを先に生成（決済処理で必要）
	orderID := nextOrderID

	// 決済処理を実行（在庫減算前）
	paymentResult := paymentGateway.ProcessPayment(totalPrice, orderID)

	// 注文オブジェクトを作成
	order := &Order{
		ID:             orderID,
		UserID:         user.ID,
		Items:          req.Items,
		TotalPrice:     totalPrice,
		ShippingFee:    shippingFee,
		DiscountAmount: discountAmount,
		AppliedCoupon:  req.CouponCode,
		CreatedAt:      time.Now(),
	}

	if paymentResult.Success {
		// 決済成功時のみ在庫を減らす
		allAllocated := true
		for _, item := range req.Items {
			allocated, allocations := allocateStock(item.ProductID, item.Quantity)
			if !allocated {
				allAllocated = false
				break
			}
			stockAllocations[item.ProductID] = allocations
		}

		if !allAllocated {
			// 在庫割り当て失敗（競合状態などで発生する可能性あり）
			order.Status = "payment_failed"
			orderMux.Lock()
			nextOrderID++
			orders[order.ID] = order
			orderMux.Unlock()
			errorResponse(w, http.StatusConflict, "Stock allocation failed. Please retry.")
			return
		}

		order.Status = "completed"

		// 注文を保存
		orderMux.Lock()
		nextOrderID++
		orders[order.ID] = order
		orderMux.Unlock()

		// 成功レスポンスにトランザクションIDを含める
		response := struct {
			*Order
			TransactionID string `json:"transaction_id"`
		}{
			Order:         order,
			TransactionID: paymentResult.TransactionID,
		}

		jsonResponse(w, http.StatusCreated, response)
	} else {
		// 決済失敗時は在庫を減らさない
		order.Status = "payment_failed"

		// 失敗した注文も記録（監査目的）
		orderMux.Lock()
		nextOrderID++
		orders[order.ID] = order
		orderMux.Unlock()

		// エラーレスポンス
		errorResponse(w, http.StatusPaymentRequired,
			fmt.Sprintf("Payment failed: %s", paymentResult.Message))
	}
}

// 注文一覧取得（ユーザー自身の注文のみ）
func getOrdersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 認証確認
	user := getAuthUser(r)
	if user == nil {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	orderMux.RLock()
	defer orderMux.RUnlock()

	var userOrders []*Order
	for _, order := range orders {
		if order.UserID == user.ID {
			userOrders = append(userOrders, order)
		}
	}

	jsonResponse(w, http.StatusOK, userOrders)
}

// メインハンドラー
func mainHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// ルーティング
	switch {
	case path == "/products" && r.Method == "GET":
		getProductsHandler(w, r)
	case path == "/products" && r.Method == "POST":
		createProductHandler(w, r)
	case strings.HasPrefix(path, "/products/") && r.Method == "GET":
		getProductHandler(w, r)
	case path == "/register" && r.Method == "POST":
		registerHandler(w, r)
	case path == "/login" && r.Method == "POST":
		loginHandler(w, r)
	case path == "/orders" && r.Method == "POST":
		createOrderHandler(w, r)
	case path == "/orders" && r.Method == "GET":
		getOrdersHandler(w, r)
	default:
		errorResponse(w, http.StatusNotFound, "Not found")
	}
}

func main() {
	port := "8081"

	fmt.Printf("Starting EC Backend API server on port %s\n", port)
	fmt.Println("\nAvailable endpoints:")
	fmt.Println("  GET    /products              - List all products (filter: ?category=xxx)")
	fmt.Println("  GET    /products/{id}         - Get product details")
	fmt.Println("  POST   /products              - Create product (admin only)")
	fmt.Println("  POST   /register              - Register new user")
	fmt.Println("  POST   /login                 - Login")
	fmt.Println("  POST   /orders                - Create order (auth required)")
	fmt.Println("  GET    /orders                - Get user's orders (auth required)")
	fmt.Println("\nDefault admin credentials: username=admin, password=admin123")

	http.HandleFunc("/", mainHandler)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
