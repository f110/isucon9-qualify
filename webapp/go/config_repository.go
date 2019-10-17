package main

import (
	"database/sql"
	"sync"

	"github.com/jmoiron/sqlx"
)

type configRepository struct {
	dbx  *sqlx.DB
	data *sync.Map
}

func NewConfigRepository(dbx *sqlx.DB) *configRepository {
	return &configRepository{dbx: dbx, data: new(sync.Map)}
}

func (c *configRepository) Flush() {
	c.data = new(sync.Map)
}

func (c *configRepository) Get(name string) (string, error) {
	v, ok := c.data.Load(name)
	if ok {
		return v.(string), nil
	}

	conf := &Config{}
	err := dbx.Get(conf, "SELECT * FROM `configs` WHERE `name` = ?", name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	c.data.Store(name, conf.Val)

	return conf.Val, nil
}
