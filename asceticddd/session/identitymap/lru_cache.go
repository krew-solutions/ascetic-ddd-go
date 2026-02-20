package identitymap

import "container/list"

type lruEntry struct {
	key   any
	value any
}

type lruCache struct {
	items map[any]*list.Element
	order *list.List
	size  int
}

func newLruCache(size int) *lruCache {
	return &lruCache{
		items: make(map[any]*list.Element, size),
		order: list.New(),
		size:  size,
	}
}

func (c *lruCache) add(key, value any) {
	if elem, ok := c.items[key]; ok {
		elem.Value = lruEntry{key: key, value: value}
		c.order.MoveToBack(elem)
		return
	}
	elem := c.order.PushBack(lruEntry{key: key, value: value})
	c.items[key] = elem
	if len(c.items) > c.size {
		front := c.order.Front()
		c.order.Remove(front)
		delete(c.items, front.Value.(lruEntry).key)
	}
}

func (c *lruCache) get(key any) (any, bool) {
	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToBack(elem)
	return elem.Value.(lruEntry).value, true
}

func (c *lruCache) remove(key any) {
	elem, ok := c.items[key]
	if !ok {
		return
	}
	delete(c.items, key)
	c.order.Remove(elem)
}

func (c *lruCache) has(key any) bool {
	_, ok := c.items[key]
	return ok
}

func (c *lruCache) clear() {
	c.items = make(map[any]*list.Element, c.size)
	c.order.Init()
}

func (c *lruCache) setSize(size int) {
	c.size = size
}
