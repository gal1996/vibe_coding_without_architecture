package main

import (
	"encoding/json"
	"fmt"
	"log"
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
	Stock    int    `json:"stock"`
	Category string `json:"category"`
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
	ID          int         `json:"id"`
	UserID      int         `json:"user_id"`
	Items       []OrderItem `json:"items"`
	TotalPrice  int         `json:"total_price"`
	ShippingFee int         `json:"shipping_fee"`
	Status      string      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
}

// データストア（インメモリ）
var (
	products    = make(map[int]*Product)
	users       = make(map[int]*User)
	usersByName = make(map[string]*User)
	orders      = make(map[int]*Order)
	sessions    = make(map[string]*User)

	productMux sync.RWMutex
	userMux    sync.RWMutex
	orderMux   sync.RWMutex
	sessionMux sync.RWMutex

	nextProductID = 1
	nextUserID    = 1
	nextOrderID   = 1
)

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

	// サンプル商品を追加
	sampleProducts := []Product{
		{ID: nextProductID, Name: "ノートPC", Price: 120000, Stock: 10, Category: "電子機器"},
		{ID: nextProductID + 1, Name: "マウス", Price: 3000, Stock: 50, Category: "電子機器"},
		{ID: nextProductID + 2, Name: "デスク", Price: 25000, Stock: 5, Category: "家具"},
		{ID: nextProductID + 3, Name: "チェア", Price: 15000, Stock: 8, Category: "家具"},
	}

	for _, p := range sampleProducts {
		product := p
		products[product.ID] = &product
	}
	nextProductID += 4
}

// ユーティリティ関数
func generateToken() string {
	return fmt.Sprintf("token_%d_%d", time.Now().Unix(), nextUserID)
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

	var result []*Product
	for _, p := range products {
		if category == "" || p.Category == category {
			result = append(result, p)
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

	jsonResponse(w, http.StatusOK, product)
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

	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// バリデーション
	if product.Name == "" || product.Price <= 0 || product.Stock < 0 || product.Category == "" {
		errorResponse(w, http.StatusBadRequest, "Invalid product data")
		return
	}

	productMux.Lock()
	product.ID = nextProductID
	nextProductID++
	products[product.ID] = &product
	productMux.Unlock()

	jsonResponse(w, http.StatusCreated, product)
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
		Items []OrderItem `json:"items"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Items) == 0 {
		errorResponse(w, http.StatusBadRequest, "No items in order")
		return
	}

	// アトミックな注文処理のためにロックを取得
	productMux.Lock()
	defer productMux.Unlock()

	// 在庫チェックと合計金額計算（税抜）
	subtotal := 0
	orderProducts := make([]*Product, len(req.Items))

	for i, item := range req.Items {
		if item.Quantity <= 0 {
			errorResponse(w, http.StatusBadRequest, fmt.Sprintf("Invalid quantity for item %d", i))
			return
		}

		product := products[item.ProductID]
		if product == nil {
			errorResponse(w, http.StatusNotFound, fmt.Sprintf("Product %d not found", item.ProductID))
			return
		}

		if product.Stock < item.Quantity {
			errorResponse(w, http.StatusBadRequest,
				fmt.Sprintf("Insufficient stock for product %s (available: %d, requested: %d)",
					product.Name, product.Stock, item.Quantity))
			return
		}

		orderProducts[i] = product
		subtotal += product.Price * item.Quantity
	}

	// 消費税計算（10%）
	tax := subtotal / 10

	// 送料計算（5000円未満の場合は500円、5000円以上は無料）
	shippingFee := 0
	if subtotal < 5000 {
		shippingFee = 500
	}

	// 合計金額（税込 + 送料）
	totalPrice := subtotal + tax + shippingFee

	// 在庫を減らす
	for i, item := range req.Items {
		orderProducts[i].Stock -= item.Quantity
	}

	// 注文を作成
	order := &Order{
		ID:          nextOrderID,
		UserID:      user.ID,
		Items:       req.Items,
		TotalPrice:  totalPrice,
		ShippingFee: shippingFee,
		Status:      "confirmed",
		CreatedAt:   time.Now(),
	}

	orderMux.Lock()
	nextOrderID++
	orders[order.ID] = order
	orderMux.Unlock()

	jsonResponse(w, http.StatusCreated, order)
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
