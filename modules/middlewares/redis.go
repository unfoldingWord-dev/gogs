// Copyright 2013 Beego Authors
// Copyright 2014 The Macaron Authors
// Copyright 2020 The Gitea Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package middlewares

import (
	"fmt"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/nosql"

	"gitea.com/go-chi/session"
	"github.com/go-redis/redis/v7"
)

// RedisStore represents a redis session store implementation.
// TODO: copied from modules/session/redis.go and should remove that one until macaron removed.
type RedisStore struct {
	c           redis.UniversalClient
	prefix, sid string
	duration    time.Duration
	lock        sync.RWMutex
	data        map[interface{}]interface{}
}

// NewRedisStore creates and returns a redis session store.
func NewRedisStore(c redis.UniversalClient, prefix, sid string, dur time.Duration, kv map[interface{}]interface{}) *RedisStore {
	return &RedisStore{
		c:        c,
		prefix:   prefix,
		sid:      sid,
		duration: dur,
		data:     kv,
	}
}

// Set sets value to given key in session.
func (s *RedisStore) Set(key, val interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.data[key] = val
	return nil
}

// Get gets value by given key in session.
func (s *RedisStore) Get(key interface{}) interface{} {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.data[key]
}

// Delete delete a key from session.
func (s *RedisStore) Delete(key interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.data, key)
	return nil
}

// ID returns current session ID.
func (s *RedisStore) ID() string {
	return s.sid
}

// Release releases resource and save data to provider.
func (s *RedisStore) Release() error {
	// Skip encoding if the data is empty
	if len(s.data) == 0 {
		return nil
	}

	data, err := session.EncodeGob(s.data)
	if err != nil {
		return err
	}

	return s.c.Set(s.prefix+s.sid, string(data), s.duration).Err()
}

// Flush deletes all session data.
func (s *RedisStore) Flush() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.data = make(map[interface{}]interface{})
	return nil
}

// RedisProvider represents a redis session provider implementation.
type RedisProvider struct {
	c        redis.UniversalClient
	duration time.Duration
	prefix   string
}

// Init initializes redis session provider.
// configs: network=tcp,addr=:6379,password=macaron,db=0,pool_size=100,idle_timeout=180,prefix=session;
func (p *RedisProvider) Init(maxlifetime int64, configs string) (err error) {
	p.duration, err = time.ParseDuration(fmt.Sprintf("%ds", maxlifetime))
	if err != nil {
		return err
	}

	uri := nosql.ToRedisURI(configs)

	for k, v := range uri.Query() {
		switch k {
		case "prefix":
			p.prefix = v[0]
		}
	}

	p.c = nosql.GetManager().GetRedisClient(uri.String())
	return p.c.Ping().Err()
}

// Read returns raw session store by session ID.
func (p *RedisProvider) Read(sid string) (session.RawStore, error) {
	psid := p.prefix + sid
	if !p.Exist(sid) {
		if err := p.c.Set(psid, "", p.duration).Err(); err != nil {
			return nil, err
		}
	}

	var kv map[interface{}]interface{}
	kvs, err := p.c.Get(psid).Result()
	if err != nil {
		return nil, err
	}
	if len(kvs) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		kv, err = session.DecodeGob([]byte(kvs))
		if err != nil {
			return nil, err
		}
	}

	return NewRedisStore(p.c, p.prefix, sid, p.duration, kv), nil
}

// Exist returns true if session with given ID exists.
func (p *RedisProvider) Exist(sid string) bool {
	v, err := p.c.Exists(p.prefix + sid).Result()
	return err == nil && v == 1
}

// Destroy deletes a session by session ID.
func (p *RedisProvider) Destroy(sid string) error {
	return p.c.Del(p.prefix + sid).Err()
}

// Regenerate regenerates a session store from old session ID to new one.
func (p *RedisProvider) Regenerate(oldsid, sid string) (_ session.RawStore, err error) {
	poldsid := p.prefix + oldsid
	psid := p.prefix + sid

	if p.Exist(sid) {
		return nil, fmt.Errorf("new sid '%s' already exists", sid)
	} else if !p.Exist(oldsid) {
		// Make a fake old session.
		if err = p.c.Set(poldsid, "", p.duration).Err(); err != nil {
			return nil, err
		}
	}

	if err = p.c.Rename(poldsid, psid).Err(); err != nil {
		return nil, err
	}

	var kv map[interface{}]interface{}
	kvs, err := p.c.Get(psid).Result()
	if err != nil {
		return nil, err
	}

	if len(kvs) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		kv, err = session.DecodeGob([]byte(kvs))
		if err != nil {
			return nil, err
		}
	}

	return NewRedisStore(p.c, p.prefix, sid, p.duration, kv), nil
}

// Count counts and returns number of sessions.
func (p *RedisProvider) Count() int {
	return int(p.c.DBSize().Val())
}

// GC calls GC to clean expired sessions.
func (*RedisProvider) GC() {}

func init() {
	session.Register("redis", &RedisProvider{})
}
