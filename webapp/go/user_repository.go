package main

import (
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
}

type userContext struct {
	data *sync.Map
}

func (c *userContext) Get(id int64) (*User, bool) {
	v, ok := c.data.Load(id)
	if !ok {
		return nil, false
	}

	return v.(*User), true
}

func (c *userContext) Set(user *User) {
	c.data.Store(user.ID, user)
}

func NewUserRepository(dbx *sqlx.DB, client *memcache.Client) *userRepository {
	return &userRepository{
		dbx:    dbx,
		client: client,
	}
}

func (u *userRepository) GetContext() *userContext {
	return &userContext{data: &sync.Map{}}
}

func (u *userRepository) Get(ctx *userContext, id int64) (*User, error) {
	if ctx != nil {
		if v, ok := ctx.Get(id); ok {
			return v, nil
		}
	}

	item, err := u.client.Get(u.key(id))
	if err != nil && err != memcache.ErrCacheMiss {
		log.Print(err)
		return nil, err
	}

	if item != nil {
		user, err := u.decode(item)
		if err != nil {
			return nil, err
		}
		if ctx != nil {
			ctx.Set(user)
		}
		return user, nil
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

	if ctx != nil {
		ctx.Set(user)
	}
	return user, nil
}

func (u *userRepository) GetMulti(ids ...int64) (map[int64]*User, error) {
	unique := make(map[int64]struct{})
	for _, i := range ids {
		unique[i] = struct{}{}
	}

	keys := make([]string, 0, len(unique))
	for v, _ := range unique {
		keys = append(keys, u.key(v))
	}
	items, err := u.client.GetMulti(keys)
	if err != nil {
		return nil, err
	}

	res := make(map[int64]*User)
	for _, v := range items {
		c, err := u.decode(v)
		if err != nil {
			return nil, err
		}
		res[c.ID] = c
	}

	if len(res) != len(unique) {
		willFetchIds := make([]int64, 0, len(unique))
		for _, v := range ids {
			if _, ok := res[v]; !ok {
				willFetchIds = append(willFetchIds, v)
			}
		}
		if len(willFetchIds) == 0 {
			return res, nil
		}

		log.Print("SELECT DB")
		q, args, err := sqlx.In("SELECT * FROM `users` WHERE `id` IN (?)", willFetchIds)
		if err != nil {
			return nil, err
		}
		users := make([]*User, 0)
		if err := dbx.Select(&users, q, args...); err != nil {
			return nil, err
		}

		for _, user := range users {
			res[user.ID] = user
			if err := u.setCache(user); err != nil {
				return nil, err
			}
		}
	}

	return res, nil
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

func (u *userRepository) decode(item *memcache.Item) (*User, error) {
	c := &UserCache{}
	if err := c.Unmarshal(item.Value); err != nil {
		return nil, err
	}

	l, _ := types.TimestampFromProto(c.LastBump)
	ca, _ := types.TimestampFromProto(c.CreatedAt)
	user := &User{
		ID:             c.Id,
		AccountName:    c.AccountName,
		HashedPassword: c.HashedPassword,
		Address:        c.Address,
		NumSellItems:   int(c.NumSellItems),
		LastBump:       l,
		CreatedAt:      ca,
	}
	return user, nil
}
