package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/jmoiron/sqlx"
)

type userRepository struct {
	dbx    *sqlx.DB
	client *memcache.Client

	bufPool *sync.Pool
}

func NewUserRepository(dbx *sqlx.DB, client *memcache.Client) *userRepository {
	return &userRepository{
		dbx:    dbx,
		client: client,
		bufPool: &sync.Pool{New: func() interface{} {
			return new(bytes.Buffer)
		}},
	}
}

func (u *userRepository) Get(id int64) (*User, error) {
	item, err := u.client.Get(u.key(id))
	if err != nil && err != memcache.ErrCacheMiss {
		log.Print(err)
		return nil, err
	}

	user := &User{}
	if item != nil {
		if err := gob.NewDecoder(bytes.NewReader(item.Value)).Decode(user); err != nil {
			return nil, err
		}
		return user, nil
	}

	err = dbx.Get(user, "SELECT * FROM `users` WHERE `id` = ?", id)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	if err := u.setCache(user); err != nil {
		log.Print(err)
		return nil, err
	}

	return user, nil
}

func (u *userRepository) UpdateCache(user *User) error {
	return u.setCache(user)
}

func (u *userRepository) Invalidate(id int64) error {
	return u.client.Delete(u.key(id))
}

func (u *userRepository) Flush() {
	u.client.FlushAll()
}

func (u *userRepository) LastBump(id int64) error {
	_, err := u.client.Get(u.bumpKey(id))
	if err != nil {
		return nil
	}

	return errors.New("user bumped recently")
}

func (u *userRepository) Bump(id int64) error {
	return u.client.Set(&memcache.Item{Key: u.bumpKey(id), Value: []byte("1"), Expiration: BumpChargeSeconds})
}

func (u *userRepository) setCache(user *User) error {
	buf := u.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		u.bufPool.Put(buf)
	}()

	if err := gob.NewEncoder(buf).Encode(user); err != nil {
		return nil
	}

	return u.client.Set(&memcache.Item{Key: u.key(user.ID), Value: buf.Bytes()})
}

func (u *userRepository) key(id int64) string {
	return fmt.Sprintf("user/%d", id)
}

func (u *userRepository) bumpKey(id int64) string {
	return fmt.Sprintf("user_dump/%d", id)
}
