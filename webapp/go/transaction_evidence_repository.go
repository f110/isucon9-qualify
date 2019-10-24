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

type transactionEvidenceRepository struct {
	dbx    *sqlx.DB
	client *memcache.Client

	bufPool *sync.Pool
}

func NewTransactionEvidenceRepository(dbx *sqlx.DB, client *memcache.Client) *transactionEvidenceRepository {
	return &transactionEvidenceRepository{
		dbx:    dbx,
		client: client,
		bufPool: &sync.Pool{New: func() interface{} {
			return new(bytes.Buffer)
		}},
	}
}

func (t *transactionEvidenceRepository) Get(itemId int64) (*TransactionEvidence, error) {
	item, err := t.client.Get(t.key(itemId))
	if err != nil && err != memcache.ErrCacheMiss {
		return nil, err
	}

	if item != nil {
		c := &TransactionEvidenceCache{}
		if err := c.Unmarshal(item.Value); err != nil {
			return nil, err
		}

		ca, _ := types.TimestampFromProto(c.CreatedAt)
		ua, _ := types.TimestampFromProto(c.UpdatedAt)
		return &TransactionEvidence{
			ID:                 c.Id,
			SellerID:           c.SellerId,
			BuyerID:            c.BuyerId,
			Status:             c.Status,
			ItemID:             c.ItemId,
			ItemName:           c.ItemName,
			ItemPrice:          int(c.ItemPrice),
			ItemDescription:    c.ItemDescription,
			ItemCategoryID:     int(c.ItemCategoryId),
			ItemRootCategoryID: int(c.ItemRootCategoryId),
			CreatedAt:          ca,
			UpdatedAt:          ua,
		}, nil
	}

	transactionEvidence := &TransactionEvidence{}
	err = dbx.Get(transactionEvidence, "SELECT * FROM `transaction_evidences` WHERE `item_id` = ?", itemId)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	if err := t.setCache(transactionEvidence); err != nil {
		log.Print(err)
		return nil, err
	}

	return transactionEvidence, nil
}

func (t *transactionEvidenceRepository) UpdateCache(transactionEvidence *TransactionEvidence) error {
	return t.setCache(transactionEvidence)
}

func (t *transactionEvidenceRepository) Invalidate(id int64) error {
	return t.client.Delete(t.key(id))
}

func (t *transactionEvidenceRepository) Flush() {
	t.client.Flush(0)
}

func (t *transactionEvidenceRepository) setCache(transactionEvidence *TransactionEvidence) error {
	ca, _ := types.TimestampProto(transactionEvidence.CreatedAt)
	ua, _ := types.TimestampProto(transactionEvidence.UpdatedAt)
	c := &TransactionEvidenceCache{
		Id:                 transactionEvidence.ID,
		SellerId:           transactionEvidence.SellerID,
		BuyerId:            transactionEvidence.BuyerID,
		Status:             transactionEvidence.Status,
		ItemId:             transactionEvidence.ItemID,
		ItemName:           transactionEvidence.ItemName,
		ItemPrice:          int32(transactionEvidence.ItemPrice),
		ItemDescription:    transactionEvidence.ItemDescription,
		ItemCategoryId:     int32(transactionEvidence.ItemCategoryID),
		ItemRootCategoryId: int32(transactionEvidence.ItemRootCategoryID),
		CreatedAt:          ca,
		UpdatedAt:          ua,
	}

	b, err := c.Marshal()
	if err != nil {
		return err
	}
	return t.client.Set(&memcache.Item{Key: t.key(transactionEvidence.ItemID), Value: b})
}

func (t *transactionEvidenceRepository) key(id int64) string {
	return fmt.Sprintf("transaction_evidence/%d", id)
}
