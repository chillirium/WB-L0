package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go-kafka-postgres/internal/cache"
	"go-kafka-postgres/internal/db"
	"go-kafka-postgres/internal/logger"
	"go-kafka-postgres/internal/model"

	"github.com/IBM/sarama"
)

// Consumer представляет потребителя Kafka для обработки заказов
type Consumer struct {
	consumer sarama.ConsumerGroup
	cache    cache.Cache
	db       db.DatabaseInterface
	topic    string
	groupID  string
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// New создает нового потребителя Kafka (ConsumerGroup)
func New(brokers []string, topic string, cache cache.Cache, db db.DatabaseInterface) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Offsets.AutoCommit.Enable = true

	groupID := "orders-consumer-group"

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		consumer: consumer,
		cache:    cache,
		db:       db,
		topic:    topic,
		groupID:  groupID,
		stopChan: make(chan struct{}),
	}, nil
}

// Start начинает потребление сообщений
func (c *Consumer) Start() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		handler := &consumerHandler{
			cache: c.cache,
			db:    c.db,
		}
		for {
			if err := c.consumer.Consume(context.Background(), []string{c.topic}, handler); err != nil {
				logger.Errorf("Consumer error: %v", err)
			}
			select {
			case <-c.stopChan:
				return
			default:
			}
		}
	}()
	logger.Infof("Started Kafka consumer group %s for topic %s", c.groupID, c.topic)
}

// consumerHandler реализует sarama.ConsumerGroupHandler
type consumerHandler struct {
	cache cache.Cache
	db    db.DatabaseInterface
}

func (h *consumerHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		logger.Infof("Received message from partition %d at offset %d", message.Partition, message.Offset)

		var order model.Order
		if err := json.Unmarshal(message.Value, &order); err != nil {
			logger.Errorf("Failed to unmarshal order: %v. Message: %s", err, string(message.Value))
			session.MarkMessage(message, "")
			continue
		}

		if err := validateOrder(&order); err != nil {
			logger.Errorf("Invalid order %s: %v. Skipping.", order.OrderUID, err)
			session.MarkMessage(message, "")
			continue
		}

		if err := h.db.InsertOrder(&order); err != nil {
			logger.Errorf("Failed to insert order %s into database: %v", order.OrderUID, err)
			continue
		}

		h.cache.Set(&order)
		logger.Infof("Order %s processed successfully", order.OrderUID)

		session.MarkMessage(message, "")
	}
	return nil
}

// Функция валидации
func validateOrder(order *model.Order) error {
	now := time.Now().Add(1 * time.Minute)

	if order.DateCreated.After(now) {
		return fmt.Errorf("date_created is in the future: %v", order.DateCreated)
	}

	if order.OrderUID == "" {
		return fmt.Errorf("missing order_uid")
	}
	if order.TrackNumber == "" {
		return fmt.Errorf("missing track_number")
	}
	if order.Entry == "" {
		return fmt.Errorf("missing entry")
	}
	if order.Locale == "" {
		return fmt.Errorf("missing locale")
	}
	if order.CustomerID == "" {
		return fmt.Errorf("missing customer_id")
	}
	if order.DeliveryService == "" {
		return fmt.Errorf("missing delivery_service")
	}
	if order.Shardkey == "" {
		return fmt.Errorf("missing shardkey")
	}
	if order.OofShard == "" {
		return fmt.Errorf("missing oof_shard")
	}

	if order.Delivery.Name == "" || order.Delivery.Phone == "" || order.Delivery.Zip == "" ||
		order.Delivery.City == "" || order.Delivery.Address == "" || order.Delivery.Region == "" ||
		order.Delivery.Email == "" {
		return fmt.Errorf("missing fields in delivery")
	}

	if order.Payment.Transaction == "" || order.Payment.Currency == "" || order.Payment.Provider == "" ||
		order.Payment.Bank == "" {
		return fmt.Errorf("missing fields in payment")
	}
	if order.Payment.Amount <= 0 || order.Payment.PaymentDt <= 0 || order.Payment.DeliveryCost < 0 ||
		order.Payment.GoodsTotal <= 0 || order.Payment.CustomFee < 0 {
		return fmt.Errorf("invalid numeric values in payment")
	}

	if len(order.Items) == 0 {
		return fmt.Errorf("no items")
	}
	for i, item := range order.Items {
		if item.ChrtID == 0 || item.TrackNumber == "" || item.Price <= 0 || item.Rid == "" ||
			item.Name == "" || item.Sale < 0 || item.Size == "" || item.TotalPrice <= 0 ||
			item.NmID == 0 || item.Brand == "" || item.Status <= 0 {
			return fmt.Errorf("missing/invalid fields in item #%d", i+1)
		}
	}

	return nil
}

// Close закрывает потребителя
func (c *Consumer) Close() error {
	close(c.stopChan)
	c.wg.Wait()
	return c.consumer.Close()
}
