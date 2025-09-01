package db

import (
	"context"
	"fmt"
	"go-kafka-postgres/internal/logger"
	"go-kafka-postgres/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseInterface interface {
	InsertOrder(order *model.Order) error
	GetAllOrders() ([]*model.Order, error)
	GetOrderByUID(uid string) (*model.Order, error)
	Close()
}

type Database struct {
	pool *pgxpool.Pool
}

// New создает новое подключение к базе данных
func New(connString string) (*Database, error) {
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return &Database{pool: pool}, nil
}

// Close закрывает пул соединений с базой данных
func (db *Database) Close() {
	db.pool.Close()
}

// InsertOrder вставляет новый заказ в базу данных в транзакции
func (db *Database) InsertOrder(order *model.Order) error {
	ctx := context.Background()

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction error: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	orderQuery := `INSERT INTO orders (
		order_uid, track_number, entry, locale, internal_signature,
		customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	ON CONFLICT (order_uid) DO NOTHING`

	_, err = tx.Exec(ctx, orderQuery,
		order.OrderUID,
		order.TrackNumber,
		order.Entry,
		order.Locale,
		order.InternalSignature,
		order.CustomerID,
		order.DeliveryService,
		order.Shardkey,
		order.SmID,
		order.DateCreated,
		order.OofShard,
	)
	if err != nil {
		return fmt.Errorf("insert order error: %w", err)
	}

	deliveryQuery := `INSERT INTO delivery (
		order_uid, name, phone, zip, city, address, region, email
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (order_uid) DO NOTHING`

	_, err = tx.Exec(ctx, deliveryQuery,
		order.OrderUID,
		order.Delivery.Name,
		order.Delivery.Phone,
		order.Delivery.Zip,
		order.Delivery.City,
		order.Delivery.Address,
		order.Delivery.Region,
		order.Delivery.Email,
	)
	if err != nil {
		return fmt.Errorf("insert delivery error: %w", err)
	}

	paymentQuery := `INSERT INTO payment (
		order_uid, transaction, request_id, currency, provider,
		amount, payment_dt, bank, delivery_cost, goods_total, custom_fee
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	ON CONFLICT (order_uid) DO NOTHING`

	_, err = tx.Exec(ctx, paymentQuery,
		order.OrderUID,
		order.Payment.Transaction,
		order.Payment.RequestID,
		order.Payment.Currency,
		order.Payment.Provider,
		order.Payment.Amount,
		order.Payment.PaymentDt,
		order.Payment.Bank,
		order.Payment.DeliveryCost,
		order.Payment.GoodsTotal,
		order.Payment.CustomFee,
	)
	if err != nil {
		return fmt.Errorf("insert payment error: %w", err)
	}

	itemQuery := `INSERT INTO items (
		order_uid, chrt_id, track_number, price, rid, name,
		sale, size, total_price, nm_id, brand, status
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	ON CONFLICT (order_uid, chrt_id) DO NOTHING`

	for _, item := range order.Items {
		_, err = tx.Exec(ctx, itemQuery,
			order.OrderUID,
			item.ChrtID,
			item.TrackNumber,
			item.Price,
			item.Rid,
			item.Name,
			item.Sale,
			item.Size,
			item.TotalPrice,
			item.NmID,
			item.Brand,
			item.Status,
		)
		if err != nil {
			return fmt.Errorf("insert item error: %w", err)
		}
	}

	// Фиксируем транзакцию
	return tx.Commit(ctx)
}

// GetAllOrders извлекает все заказы из базы данных
func (db *Database) GetAllOrders() ([]*model.Order, error) {
	ctx := context.Background()

	query := `
		SELECT 
			o.order_uid, o.track_number, o.entry, o.locale, o.internal_signature,
			o.customer_id, o.delivery_service, o.shardkey, o.sm_id, o.date_created, o.oof_shard,
			d.name, d.phone, d.zip, d.city, d.address, d.region, d.email,
			p.transaction, p.request_id, p.currency, p.provider, p.amount, p.payment_dt,
			p.bank, p.delivery_cost, p.goods_total, p.custom_fee
		FROM orders o
		LEFT JOIN delivery d ON o.order_uid = d.order_uid
		LEFT JOIN payment p ON o.order_uid = p.order_uid
	`

	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query orders error: %w", err)
	}
	defer rows.Close()

	ordersMap := make(map[string]*model.Order)
	for rows.Next() {
		var order model.Order
		var delivery model.Delivery
		var payment model.Payment

		err := rows.Scan(
			&order.OrderUID,
			&order.TrackNumber,
			&order.Entry,
			&order.Locale,
			&order.InternalSignature,
			&order.CustomerID,
			&order.DeliveryService,
			&order.Shardkey,
			&order.SmID,
			&order.DateCreated,
			&order.OofShard,
			&delivery.Name,
			&delivery.Phone,
			&delivery.Zip,
			&delivery.City,
			&delivery.Address,
			&delivery.Region,
			&delivery.Email,
			&payment.Transaction,
			&payment.RequestID,
			&payment.Currency,
			&payment.Provider,
			&payment.Amount,
			&payment.PaymentDt,
			&payment.Bank,
			&payment.DeliveryCost,
			&payment.GoodsTotal,
			&payment.CustomFee,
		)
		if err != nil {
			logger.Errorf("Error scanning order: %v", err)
			continue
		}

		order.Delivery = delivery
		order.Payment = payment
		ordersMap[order.OrderUID] = &order
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	itemsQuery := `SELECT order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status FROM items`
	itemsRows, err := db.pool.Query(ctx, itemsQuery)
	if err != nil {
		return nil, fmt.Errorf("query items error: %w", err)
	}
	defer itemsRows.Close()

	for itemsRows.Next() {
		var item model.Item
		var orderUID string

		err := itemsRows.Scan(
			&orderUID,
			&item.ChrtID,
			&item.TrackNumber,
			&item.Price,
			&item.Rid,
			&item.Name,
			&item.Sale,
			&item.Size,
			&item.TotalPrice,
			&item.NmID,
			&item.Brand,
			&item.Status,
		)
		if err != nil {
			logger.Errorf("Error scanning item: %v", err)
			continue
		}

		if order, exists := ordersMap[orderUID]; exists {
			order.Items = append(order.Items, item)
		}
	}

	if err := itemsRows.Err(); err != nil {
		return nil, fmt.Errorf("items rows iteration error: %w", err)
	}

	orders := make([]*model.Order, 0, len(ordersMap))
	for _, order := range ordersMap {
		orders = append(orders, order)
	}

	return orders, nil
}

// GetOrderByUID извлекает конкретный заказ по его UID
func (db *Database) GetOrderByUID(uid string) (*model.Order, error) {
	ctx := context.Background()

	query := `
		SELECT 
			o.order_uid, o.track_number, o.entry, o.locale, o.internal_signature,
			o.customer_id, o.delivery_service, o.shardkey, o.sm_id, o.date_created, o.oof_shard,
			d.name, d.phone, d.zip, d.city, d.address, d.region, d.email,
			p.transaction, p.request_id, p.currency, p.provider, p.amount, p.payment_dt,
			p.bank, p.delivery_cost, p.goods_total, p.custom_fee
		FROM orders o
		LEFT JOIN delivery d ON o.order_uid = d.order_uid
		LEFT JOIN payment p ON o.order_uid = p.order_uid
		WHERE o.order_uid = $1
	`

	var order model.Order
	var delivery model.Delivery
	var payment model.Payment

	err := db.pool.QueryRow(ctx, query, uid).Scan(
		&order.OrderUID,
		&order.TrackNumber,
		&order.Entry,
		&order.Locale,
		&order.InternalSignature,
		&order.CustomerID,
		&order.DeliveryService,
		&order.Shardkey,
		&order.SmID,
		&order.DateCreated,
		&order.OofShard,
		&delivery.Name,
		&delivery.Phone,
		&delivery.Zip,
		&delivery.City,
		&delivery.Address,
		&delivery.Region,
		&delivery.Email,
		&payment.Transaction,
		&payment.RequestID,
		&payment.Currency,
		&payment.Provider,
		&payment.Amount,
		&payment.PaymentDt,
		&payment.Bank,
		&payment.DeliveryCost,
		&payment.GoodsTotal,
		&payment.CustomFee,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("order not found")
		}
		return nil, fmt.Errorf("query order error: %w", err)
	}

	order.Delivery = delivery
	order.Payment = payment

	itemsQuery := `SELECT chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status FROM items WHERE order_uid = $1`
	itemsRows, err := db.pool.Query(ctx, itemsQuery, uid)
	if err != nil {
		return nil, fmt.Errorf("query items error: %w", err)
	}
	defer itemsRows.Close()

	for itemsRows.Next() {
		var item model.Item
		err := itemsRows.Scan(
			&item.ChrtID,
			&item.TrackNumber,
			&item.Price,
			&item.Rid,
			&item.Name,
			&item.Sale,
			&item.Size,
			&item.TotalPrice,
			&item.NmID,
			&item.Brand,
			&item.Status,
		)
		if err != nil {
			logger.Errorf("Error scanning item: %v", err)
			continue
		}
		order.Items = append(order.Items, item)
	}

	if err := itemsRows.Err(); err != nil {
		return nil, fmt.Errorf("items rows iteration error: %w", err)
	}

	return &order, nil
}
