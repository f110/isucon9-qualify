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

	transactionEvidence := &TransactionEvidence{}
	if item != nil {
		if err := gob.NewDecoder(bytes.NewBuffer(item.Value)).Decode(transactionEvidence); err != nil {
			return nil, err
		}
		return transactionEvidence, nil
	}

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
	buf := t.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		t.bufPool.Put(buf)
	}()

	if err := gob.NewEncoder(buf).Encode(transactionEvidence); err != nil {
		return nil
	}

	return t.client.Set(&memcache.Item{Key: t.key(transactionEvidence.ID), Value: buf.Bytes()})
}

func (t *transactionEvidenceRepository) key(id int64) string {
	return fmt.Sprintf("transaction_evidence/%d", id)
}
