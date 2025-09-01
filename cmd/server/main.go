package main

import (
	"net/http"
	"os"

	"go-kafka-postgres/internal/cache"
	"go-kafka-postgres/internal/consumer"
	"go-kafka-postgres/internal/db"
	"go-kafka-postgres/internal/handler"
	"go-kafka-postgres/internal/logger"
)

func main() {
	if err := logger.Init(os.Getenv("LOG_LEVEL")); err != nil {
		panic("Failed to init logger: " + err.Error())
	}
	defer logger.Sync()

	connString := os.Getenv("POSTGRES_CONN_STRING")
	if connString == "" {
		connString = "postgres://user:password@localhost:5432/orders_db?sslmode=disable"
	}
	database, err := db.New(connString)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer database.Close()

	cache := cache.New(2)

	orders, err := database.GetAllOrders()
	if err != nil {
		logger.Fatal(err.Error())
	}
	cache.Restore(orders)
	logger.Infof("Restored %d orders from database", cache.Size())

	brokersEnv := os.Getenv("KAFKA_BROKERS")
	if brokersEnv == "" {
		brokersEnv = "localhost:9092"
	}
	brokers := []string{brokersEnv}
	topic := "orders"
	consumer, err := consumer.New(brokers, topic, cache, database)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer consumer.Close()

	go consumer.Start()

	hand := handler.New(cache, database)

	http.HandleFunc("/order/", hand.GetOrder)
	http.Handle("/", http.FileServer(http.Dir("./web")))

	logger.Info("Server started on :8081")
	logger.Fatal(http.ListenAndServe(":8081", nil).Error())
}
