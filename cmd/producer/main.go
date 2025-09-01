package main

import (
	"encoding/json"
	"os"
	"time"

	"go-kafka-postgres/internal/logger"
	"go-kafka-postgres/internal/model"

	"github.com/IBM/sarama"
)

func main() {
	if err := logger.Init(os.Getenv("LOG_LEVEL")); err != nil {
		panic("Failed to init logger: " + err.Error())
	}
	defer logger.Sync()

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Retry.Max = 5
	config.Producer.RequiredAcks = sarama.WaitForAll

	brokers := []string{"localhost:9092"}
	if envBrokers := os.Getenv("KAFKA_BROKERS"); envBrokers != "" {
		brokers = []string{envBrokers}
	}

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		logger.Fatalf("Error creating producer: %v", err)
	}
	defer producer.Close()

	topic := "orders"
	if envTopic := os.Getenv("KAFKA_TOPIC"); envTopic != "" {
		topic = envTopic
	}

	orders, err := loadTestData()
	if err != nil {
		logger.Fatalf("Error loading test data: %v", err)
	}

	for i, order := range orders {
		messageJSON, err := json.Marshal(order)
		if err != nil {
			logger.Errorf("Error marshaling order %d: %v", i, err)
			continue
		}

		msg := &sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder(order.OrderUID),
			Value: sarama.ByteEncoder(messageJSON),
		}

		partition, offset, err := producer.SendMessage(msg)
		if err != nil {
			logger.Errorf("Error sending message %d: %v", i, err)
		} else {
			logger.Infof("Message %d sent successfully. Partition: %d, Offset: %d, OrderUID: %s",
				i, partition, offset, order.OrderUID)
		}

		time.Sleep(500 * time.Millisecond)
	}

	logger.Info("All messages sent successfully")
}

func loadTestData() ([]model.Order, error) {
	if fileData, err := os.ReadFile("model.json"); err == nil {
		var order model.Order
		if err := json.Unmarshal(fileData, &order); err == nil {
			return []model.Order{order}, nil
		} else {
			logger.Errorf("Invalid JSON in model.json: %v", err)
			return nil, nil
		}
	} else {
		logger.Errorf("Failed to read model.json: %v", err)
		return nil, nil
	}
}
