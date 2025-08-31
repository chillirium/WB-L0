package cache

import (
	"go-kafka-postgres/internal/model"
	"sync"
)

// Cache интерфейс для кэша
type Cache interface {
	Get(uid string) (*model.Order, bool)
	Set(order *model.Order)
	Restore(orders []*model.Order)
}

// lruNode узел двусвязного списка для LRU
type lruNode struct {
	key  string
	prev *lruNode
	next *lruNode
}

// OrderCache реализация кэша заказов с LRU инвалидацией
type OrderCache struct {
	mu      sync.RWMutex
	orders  map[string]*model.Order
	lruHead *lruNode
	lruTail *lruNode
	nodeMap map[string]*lruNode // Соответствие ключа узлу LRU
	maxSize int
}

// New создает новый кэш заказов с ограничением размера
func New(maxSize int) Cache {
	return &OrderCache{
		orders:  make(map[string]*model.Order),
		nodeMap: make(map[string]*lruNode),
		maxSize: maxSize,
	}
}

// Get возвращает заказ по UID и обновляет его позицию в LRU
func (c *OrderCache) Get(uid string) (*model.Order, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	order, ok := c.orders[uid]
	if ok {
		// Обновляем позицию в LRU
		c.updateLRU(uid)
	}
	return order, ok
}

// Set добавляет заказ в кэш
func (c *OrderCache) Set(order *model.Order) {
	c.mu.Lock()
	defer c.mu.Unlock()

	uid := order.OrderUID

	// Если элемент уже exists, обновляем его позицию в LRU
	if _, exists := c.orders[uid]; exists {
		c.updateLRU(uid)
		c.orders[uid] = order
		return
	}

	// Проверяем, не превысили ли мы максимальный размер
	if len(c.orders) >= c.maxSize {
		// Удаляем наименее используемый элемент
		c.evictLRU()
	}

	// Добавляем новый элемент
	c.orders[uid] = order
	c.addToLRU(uid)
}

// Restore восстанавливает кэш из списка заказов
func (c *OrderCache) Restore(orders []*model.Order) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Очищаем текущий кэш
	c.orders = make(map[string]*model.Order)
	c.nodeMap = make(map[string]*lruNode)
	c.lruHead = nil
	c.lruTail = nil

	// Восстанавливаем заказы
	for _, order := range orders {
		uid := order.OrderUID
		c.orders[uid] = order
		c.addToLRU(uid)

		// Если превысили maxSize, останавливаем восстановление
		if len(c.orders) >= c.maxSize {
			break
		}
	}
}

// Size возвращает размер кэша (для тестов)
func (c *OrderCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.orders)
}

// addToLRU добавляет новый элемент в начало LRU списка
func (c *OrderCache) addToLRU(uid string) {
	node := &lruNode{key: uid}

	if c.lruHead == nil {
		c.lruHead = node
		c.lruTail = node
	} else {
		node.next = c.lruHead
		c.lruHead.prev = node
		c.lruHead = node
	}

	c.nodeMap[uid] = node
}

// updateLRU перемещает элемент в начало LRU списка
func (c *OrderCache) updateLRU(uid string) {
	node, exists := c.nodeMap[uid]
	if !exists {
		return
	}

	// Если элемент уже в начале, ничего не делаем
	if node == c.lruHead {
		return
	}

	// Удаляем элемент из текущей позиции
	if node.prev != nil {
		node.prev.next = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	}

	// Если элемент был хвостом, обновляем хвост
	if node == c.lruTail {
		c.lruTail = node.prev
	}

	// Добавляем элемент в начало
	node.prev = nil
	node.next = c.lruHead
	if c.lruHead != nil {
		c.lruHead.prev = node
	}
	c.lruHead = node

	// Обновляем хвост если необходимо
	if c.lruTail == nil {
		c.lruTail = node
	}
}

// evictLRU удаляет наименее используемый элемент из кэша
func (c *OrderCache) evictLRU() {
	if c.lruTail == nil {
		return
	}

	// Удаляем из orders
	delete(c.orders, c.lruTail.key)

	// Удаляем из nodeMap
	delete(c.nodeMap, c.lruTail.key)

	// Обновляем хвост
	if c.lruTail.prev != nil {
		c.lruTail.prev.next = nil
		c.lruTail = c.lruTail.prev
	} else {
		// Это был последний элемент
		c.lruHead = nil
		c.lruTail = nil
	}
}
