package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/gogo/protobuf/types"

	"github.com/jmoiron/sqlx"
	"github.com/rainycape/memcache"
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

	if item != nil {
		c := &UserCache{}
		if err := c.Unmarshal(item.Value); err != nil {
			return nil, err
		}

		l, _ := types.TimestampFromProto(c.LastBump)
		ca, _ := types.TimestampFromProto(c.CreatedAt)
		return &User{
			ID:             c.Id,
			AccountName:    c.AccountName,
			HashedPassword: c.HashedPassword,
			Address:        c.Address,
			NumSellItems:   int(c.NumSellItems),
			LastBump:       l,
			CreatedAt:      ca,
		}, nil
	}

	user := &User{}
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
	u.client.Flush(0)
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
	l, _ := types.TimestampProto(user.LastBump)
	c, _ := types.TimestampProto(user.CreatedAt)
	v := &UserCache{
		Id:             user.ID,
		AccountName:    user.AccountName,
		HashedPassword: user.HashedPassword,
		Address:        user.Address,
		NumSellItems:   int32(user.NumSellItems),
		LastBump:       l,
		CreatedAt:      c,
	}
	b, err := v.Marshal()
	if err != nil {
		return err
	}

	return u.client.Set(&memcache.Item{Key: u.key(user.ID), Value: b})
}

func (u *userRepository) key(id int64) string {
	return fmt.Sprintf("user/%d", id)
}

func (u *userRepository) bumpKey(id int64) string {
	return fmt.Sprintf("user_dump/%d", id)
}
