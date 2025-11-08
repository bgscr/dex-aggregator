package aggregator

import (
	"context"
	"fmt"
	"log"
	"strings"

	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"
)

type PathFinder struct {
	cache   cache.Store
	maxHops int
}

// NewPathFinder 不再需要 baseTokens
func NewPathFinder(cache cache.Store) *PathFinder {
	return &PathFinder{
		cache:   cache,
		maxHops: 3, // 默认最大跳数
	}
}

// FindAllPaths 查找所有路径（BFS + 回溯）
func (pf *PathFinder) FindAllPaths(ctx context.Context, tokenIn, tokenOut string, maxHops int) ([][]*types.Pool, error) {
	if maxHops <= 0 {
		maxHops = pf.maxHops
	}

	normalizedTokenIn := strings.ToLower(tokenIn)
	normalizedTokenOut := strings.ToLower(tokenOut)

	log.Printf("PathFinder: Looking for paths from %s to %s (maxHops: %d)",
		normalizedTokenIn, normalizedTokenOut, maxHops)

	// 1. 获取所有池子以构建图
	allPools, err := pf.cache.GetAllPools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all pools: %v", err)
	}

	// 2. 构建邻接表 (token -> neighbors) 和 池子映射 (tokenA -> tokenB -> pools)
	adj := make(map[string]map[string]bool)
	poolMap := make(map[string]map[string][]*types.Pool)

	for _, pool := range allPools {
		t0 := pool.Token0.Address
		t1 := pool.Token1.Address

		// --- 初始化 Map ---
		if adj[t0] == nil {
			adj[t0] = make(map[string]bool)
		}
		if adj[t1] == nil {
			adj[t1] = make(map[string]bool)
		}
		if poolMap[t0] == nil {
			poolMap[t0] = make(map[string][]*types.Pool)
		}
		if poolMap[t1] == nil {
			poolMap[t1] = make(map[string][]*types.Pool)
		}
		// --- 添加边 ---
		adj[t0][t1] = true
		adj[t1][t0] = true
		poolMap[t0][t1] = append(poolMap[t0][t1], pool)
		poolMap[t1][t0] = append(poolMap[t1][t0], pool)
	}

	// 3. 执行 BFS 查找所有代币路径
	var allTokenPaths [][]string
	// 队列存储的是 "代币路径" (e.g., [tokenA, tokenB, tokenC])
	queue := [][]string{{normalizedTokenIn}}

	for len(queue) > 0 {
		currentPathTokens := queue[0]
		queue = queue[1:]

		lastToken := currentPathTokens[len(currentPathTokens)-1]

		// 找到目标
		if lastToken == normalizedTokenOut {
			allTokenPaths = append(allTokenPaths, currentPathTokens)
			// 注意：我们不在这里停止，因为我们想找到所有路径，而不仅仅是最短的
		}

		// 达到最大跳数，停止探索
		// 路径长度（代币数）= 跳数 + 1
		if len(currentPathTokens) >= maxHops+1 {
			continue
		}

		// 探索邻居
		neighbors := adj[lastToken]
		for neighbor := range neighbors {
			// 检查是否在当前路径中访问过（防止环路）
			isVisited := false
			for _, tokenInPath := range currentPathTokens {
				if tokenInPath == neighbor {
					isVisited = true
					break
				}
			}

			if !isVisited {
				// 创建新路径并加入队列
				newPathTokens := make([]string, len(currentPathTokens))
				copy(newPathTokens, currentPathTokens)
				newPathTokens = append(newPathTokens, neighbor)
				queue = append(queue, newPathTokens)
			}
		}
	} // end of: for len(queue) > 0

	log.Printf("PathFinder: Found %d token paths", len(allTokenPaths))

	// 4. 将 "代币路径" 转换为 "池子路径" (使用回溯法)
	var allPoolPaths [][]*types.Pool
	for _, tokenPath := range allTokenPaths {
		// buildPoolPaths 会找到该代币路径的所有池子组合
		poolPaths := pf.buildPoolPaths(tokenPath, poolMap)
		allPoolPaths = append(allPoolPaths, poolPaths...)
	}

	log.Printf("PathFinder: Found %d total pool paths (after combinations)", len(allPoolPaths))
	return allPoolPaths, nil
} // 这里添加了缺失的 }

// buildPoolPaths 使用回溯法将代币路径转换为所有可能的池子路径组合
// 例如：[A, B, C] -> [Pool_AB_1, Pool_BC_1], [Pool_AB_1, Pool_BC_2], [Pool_AB_2, Pool_BC_1], ...
func (pf *PathFinder) buildPoolPaths(tokens []string, poolMap map[string]map[string][]*types.Pool) [][]*types.Pool {
	var paths [][]*types.Pool
	var currentPoolPath []*types.Pool

	var backtrack func(tokenIndex int)
	backtrack = func(tokenIndex int) {
		// 基本情况：已经为所有代币对添加了池子
		if tokenIndex == len(tokens)-1 {
			// 复制一份完整的路径并存储
			finalPath := make([]*types.Pool, len(currentPoolPath))
			copy(finalPath, currentPoolPath)
			paths = append(paths, finalPath)
			return
		}

		// 递归步骤
		tokenA := tokens[tokenIndex]
		tokenB := tokens[tokenIndex+1]

		availablePools := poolMap[tokenA][tokenB]
		if len(availablePools) == 0 {
			// 如果没有池子（理论上不应该发生），则终止这条路径
			return
		}

		// 尝试此步骤的每一个可用池子
		for _, pool := range availablePools {
			// 1. 选择
			currentPoolPath = append(currentPoolPath, pool)
			// 2. 探索
			backtrack(tokenIndex + 1)
			// 3. 回溯（撤销选择）
			currentPoolPath = currentPoolPath[:len(currentPoolPath)-1]
		}
	}

	// 从第一个代币开始（索引0）
	backtrack(0)
	return paths
}
