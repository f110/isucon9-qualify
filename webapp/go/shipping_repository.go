package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"

	"github.com/gogo/protobuf/types"
	"github.com/jmoiron/sqlx"
	"github.com/rainycape/memcache"
)

type shippingRepository struct {
	dbx    *sqlx.DB
	client *memcache.Client
}

func NewShippingRepository(dbx *sqlx.DB, client *memcache.Client) *shippingRepository {
	return &shippingRepository{
		dbx:    dbx,
		client: client,
	}
}

func (s *shippingRepository) Get(transactionEvidenceId int64) (*Shipping, error) {
	item, err := s.client.Get(s.key(transactionEvidenceId))
	if err != nil && err != memcache.ErrCacheMiss {
		log.Print(err)
		return nil, err
	}

	if item != nil {
		c := &ShippingCache{}
		if err := c.Unmarshal(item.Value); err != nil {
			return nil, err
		}

		ca, _ := types.TimestampFromProto(c.CreatedAt)
		ua, _ := types.TimestampFromProto(c.UpdatedAt)
		return &Shipping{
			TransactionEvidenceID: c.TransactionEvidenceId,
			Status:                c.Status,
			ItemName:              c.ItemName,
			ItemID:                c.ItemId,
			ReserveID:             c.ReserveId,
			ReserveTime:           c.ReserveTime,
			ToAddress:             c.ToAddress,
			ToName:                c.ToName,
			FromAddress:           c.FromAddress,
			FromName:              c.FromName,
			ImgBinary:             c.ImgBinary,
			CreatedAt:             ca,
			UpdatedAt:             ua,
		}, nil
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
	ca, _ := types.TimestampProto(shipping.CreatedAt)
	ua, _ := types.TimestampProto(shipping.UpdatedAt)
	c := &ShippingCache{
		TransactionEvidenceId: shipping.TransactionEvidenceID,
		Status:                shipping.Status,
		ItemName:              shipping.ItemName,
		ItemId:                shipping.ItemID,
		ReserveId:             shipping.ReserveID,
		ReserveTime:           shipping.ReserveTime,
		ToAddress:             shipping.ToAddress,
		ToName:                shipping.ToName,
		FromAddress:           shipping.FromAddress,
		FromName:              shipping.FromName,
		ImgBinary:             shipping.ImgBinary,
		CreatedAt:             ca,
		UpdatedAt:             ua,
	}

	b, err := c.Marshal()
	if err != nil {
		return err
	}

	return s.client.Set(&memcache.Item{Key: s.key(shipping.TransactionEvidenceID), Value: b})
}

func (s *shippingRepository) key(id int64) string {
	return fmt.Sprintf("shipping/%d", id)
}
