package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/rainycape/memcache"
)

type itemRepository struct {
	dbx    *sqlx.DB
	client *memcache.Client

	bufPool *sync.Pool
}

func NewItemRepository(dbx *sqlx.DB, client *memcache.Client) *itemRepository {
	return &itemRepository{
		dbx:    dbx,
		client: client,
		bufPool: &sync.Pool{New: func() interface{} {
			return new(bytes.Buffer)
		}},
	}
}

func (i *itemRepository) Get(id int64) (*Item, error) {
	item, err := i.client.Get(i.key(id))
	if err != nil && err != memcache.ErrCacheMiss {
		return nil, err
	}

	itemObj := &Item{}
	if item != nil {
		if err := gob.NewDecoder(bytes.NewBuffer(item.Value)).Decode(itemObj); err != nil {
			return nil, err
		}
		return itemObj, nil
	}

	err = dbx.Get(itemObj, "SELECT * FROM `items` WHERE `id` = ?", id)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	if err := i.setCache(itemObj); err != nil {
		log.Print(err)
		return nil, err
	}

	return itemObj, nil
}

func (i *itemRepository) UpdateCache(item *Item) error {
	return i.setCache(item)
}

func (i *itemRepository) Invalidate(id int64) error {
	return i.client.Delete(i.key(id))
}

func (i *itemRepository) Flush() {
	i.client.Flush(0)
}

func (i *itemRepository) setCache(item *Item) error {
	buf := i.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		i.bufPool.Put(buf)
	}()

	if err := gob.NewEncoder(buf).Encode(item); err != nil {
		return nil
	}

	return i.client.Set(&memcache.Item{Key: i.key(item.ID), Value: buf.Bytes()})
}

func (i *itemRepository) key(id int64) string {
	return fmt.Sprintf("item/%d", id)
}
