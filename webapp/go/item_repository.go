package main

import (
	"bytes"
	"fmt"
	"log"
	"sync"

	"github.com/gogo/protobuf/types"

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

	if item != nil {
		c := &ItemCache{}
		if err := c.Unmarshal(item.Value); err != nil {
			return nil, err
		}
		ca, _ := types.TimestampFromProto(c.CreatedAt)
		ua, _ := types.TimestampFromProto(c.UpdatedAt)
		return &Item{
			ID:          c.Id,
			SellerID:    c.SellerId,
			BuyerID:     c.BuyerId,
			Status:      c.Status,
			Name:        c.Name,
			Price:       int(c.Price),
			Description: c.Description,
			ImageName:   c.ImageName,
			CategoryID:  int(c.CategoryId),
			CreatedAt:   ca,
			UpdatedAt:   ua,
		}, nil
	}

	itemObj := &Item{}
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
	ca, _ := types.TimestampProto(item.CreatedAt)
	ua, _ := types.TimestampProto(item.UpdatedAt)
	c := &ItemCache{
		Id:          item.ID,
		SellerId:    item.SellerID,
		BuyerId:     item.BuyerID,
		Status:      item.Status,
		Name:        item.Name,
		Price:       int32(item.Price),
		Description: item.Description,
		ImageName:   item.ImageName,
		CategoryId:  int32(item.CategoryID),
		CreatedAt:   ca,
		UpdatedAt:   ua,
	}

	b, err := c.Marshal()
	if err != nil {
		return err
	}

	return i.client.Set(&memcache.Item{Key: i.key(item.ID), Value: b})
}

func (i *itemRepository) key(id int64) string {
	return fmt.Sprintf("item/%d", id)
}
