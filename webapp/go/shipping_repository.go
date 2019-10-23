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

type shippingRepository struct {
	dbx    *sqlx.DB
	client *memcache.Client

	bufPool *sync.Pool
}

func NewShippingRepository(dbx *sqlx.DB, client *memcache.Client) *shippingRepository {
	return &shippingRepository{
		dbx:    dbx,
		client: client,
		bufPool: &sync.Pool{New: func() interface{} {
			return new(bytes.Buffer)
		}},
	}
}

func (s *shippingRepository) Get(transactionEvidenceId int64) (*Shipping, error) {
	item, err := s.client.Get(s.key(transactionEvidenceId))
	if err != nil && err != memcache.ErrCacheMiss {
		log.Print(err)
		return nil, err
	}

	shipping := &Shipping{}
	if item != nil {
		if err := gob.NewDecoder(bytes.NewReader(item.Value)).Decode(shipping); err != nil {
			return nil, err
		}
		return shipping, nil
	}

	err = dbx.Get(shipping, "SELECT * FROM `shippings` WHERE `transaction_evidence_id` = ?", transactionEvidenceId)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	if err := s.setCache(shipping); err != nil {
		log.Print(err)
		return nil, err
	}

	return shipping, nil
}

func (s *shippingRepository) UpdateCache(shipping *Shipping) error {
	return s.setCache(shipping)
}

func (s *shippingRepository) Invalidate(id int64) error {
	return s.client.Delete(s.key(id))
}

func (s *shippingRepository) Flush() {
	s.client.Flush(0)
}

func (s *shippingRepository) setCache(shipping *Shipping) error {
	buf := s.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		s.bufPool.Put(buf)
	}()

	if err := gob.NewEncoder(buf).Encode(shipping); err != nil {
		return nil
	}

	return s.client.Set(&memcache.Item{Key: s.key(shipping.TransactionEvidenceID), Value: buf.Bytes()})
}

func (s *shippingRepository) key(id int64) string {
	return fmt.Sprintf("shipping/%d", id)
}
