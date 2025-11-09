package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"dex-aggregator/internal/types"

	"github.com/go-redis/redis/v8"
)

type Store interface {
	StorePool(ctx context.Context, pool *types.Pool) error
	GetPool(ctx context.Context, address string) (*types.Pool, error)
	GetPoolsByTokens(ctx context.Context, tokenA, tokenB string) ([]*types.Pool, error)
	GetAllPools(ctx context.Context) ([]*types.Pool, error)
	StoreToken(ctx context.Context, token *types.Token) error
	GetToken(ctx context.Context, address string) (*types.Token, error)
}

type RedisStore struct {
	client *redis.Client
	prefix string
}

func NewRedisStore(addr, password string) *RedisStore {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	return &RedisStore{
		client: client,
		prefix: "dex:",
	}
}

func (rs *RedisStore) StorePool(ctx context.Context, pool *types.Pool) error {
	key := fmt.Sprintf("%spool:%s", rs.prefix, pool.Address)

	data, err := json.Marshal(pool)
	if err != nil {
		return err
	}

	// Store pool information
	err = rs.client.Set(ctx, key, data, 24*time.Hour).Err()
	if err != nil {
		return err
	}

	// Create token pair index
	tokenPairKey := fmt.Sprintf("%stoken_pair:%s:%s",
		rs.prefix, pool.Token0.Address, pool.Token1.Address)
	err = rs.client.SAdd(ctx, tokenPairKey, pool.Address).Err()
	if err != nil {
		return err
	}
	rs.client.Expire(ctx, tokenPairKey, 24*time.Hour)

	// Add to all pools set
	allPoolsKey := fmt.Sprintf("%sall_pools", rs.prefix)
	err = rs.client.SAdd(ctx, allPoolsKey, pool.Address).Err()
	if err != nil {
		return err
	}

	return nil
}

func (rs *RedisStore) GetPool(ctx context.Context, address string) (*types.Pool, error) {
	key := fmt.Sprintf("%spool:%s", rs.prefix, address)

	data, err := rs.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("pool not found: %s", address)
		}
		return nil, err
	}

	var pool types.Pool
	if err := json.Unmarshal([]byte(data), &pool); err != nil {
		return nil, err
	}

	return &pool, nil
}

func (rs *RedisStore) GetAllPools(ctx context.Context) ([]*types.Pool, error) {
	allPoolsKey := fmt.Sprintf("%sall_pools", rs.prefix)

	poolAddrs, err := rs.client.SMembers(ctx, allPoolsKey).Result()
	if err != nil {
		return nil, err
	}

	if len(poolAddrs) == 0 {
		return []*types.Pool{}, nil
	}

	// 1. 创建一个 Pipeline
	pipe := rs.client.Pipeline()

	// 2. 将所有 Get 命令加入 Pipeline
	cmds := make(map[string]*redis.StringCmd, len(poolAddrs))
	for _, addr := range poolAddrs {
		key := fmt.Sprintf("%spool:%s", rs.prefix, addr)
		cmds[addr] = pipe.Get(ctx, key)
	}

	// 3. 一次性执行所有命令
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		// 即使某些key不存在 (redis.Nil)，也不应阻断整个操作
		// 只有在发生连接错误等严重问题时才返回
		if err != redis.Nil {
			log.Printf("Redis pipeline Exec error: %v", err)
			return nil, err
		}
	}

	// 4. 处理结果
	var pools []*types.Pool
	for addr, cmd := range cmds {
		data, err := cmd.Result()
		if err != nil {
			if err != redis.Nil {
				log.Printf("Failed to get pool %s from pipeline: %v", addr, err)
			}
			// 如果key不存在或获取失败，则跳过
			continue
		}

		var pool types.Pool
		if err := json.Unmarshal([]byte(data), &pool); err != nil {
			log.Printf("Failed to unmarshal pool %s: %v", addr, err)
			continue
		}
		pools = append(pools, &pool)
	}

	return pools, nil
}

func (rs *RedisStore) StoreToken(ctx context.Context, token *types.Token) error {
	key := fmt.Sprintf("%stoken:%s", rs.prefix, token.Address)

	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	return rs.client.Set(ctx, key, data, 24*time.Hour).Err()
}

func (rs *RedisStore) GetToken(ctx context.Context, address string) (*types.Token, error) {
	key := fmt.Sprintf("%stoken:%s", rs.prefix, address)

	data, err := rs.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Return default token info
			return &types.Token{
				Address:  address,
				Symbol:   "UNKNOWN",
				Decimals: 18,
			}, nil
		}
		return nil, err
	}

	var token types.Token
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, err
	}

	return &token, nil
}

func (rs *RedisStore) GetPoolsByTokens(ctx context.Context, tokenA, tokenB string) ([]*types.Pool, error) {
	// Try both orderings
	keys := []string{
		fmt.Sprintf("%stoken_pair:%s:%s", rs.prefix, tokenA, tokenB),
		fmt.Sprintf("%stoken_pair:%s:%s", rs.prefix, tokenB, tokenA),
	}

	var poolAddrs []string
	for _, key := range keys {
		addrs, err := rs.client.SMembers(ctx, key).Result()
		if err == nil && len(addrs) > 0 {
			poolAddrs = append(poolAddrs, addrs...)
		}
	}

	var pools []*types.Pool
	for _, addr := range poolAddrs {
		pool, err := rs.GetPool(ctx, addr)
		if err == nil && pool != nil {
			pools = append(pools, pool)
		}
	}

	return pools, nil
}
