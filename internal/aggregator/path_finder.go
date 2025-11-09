// dex-aggregator/internal/aggregator/path_finder.go

package aggregator

import (
	// 导入 heap 包
	"container/heap"
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync" // 导入 sync 包

	"dex-aggregator/internal/cache"
	"dex-aggregator/internal/types"
)

type PathFinder struct {
	cache     cache.Store
	priceCalc *PriceCalculator // 1. 添加 PriceCalculator 依赖
	maxHops   int

	// 2. 添加图的内存缓存
	graphLock    sync.RWMutex
	adj          map[string]map[string]bool
	poolMap      map[string]map[string][]*types.Pool
	liquidityMap map[string]map[string]*big.Int // 还可以保留这个用于简单的启发
}

// 3. 更新构造函数
func NewPathFinder(cache cache.Store, priceCalc *PriceCalculator) *PathFinder {
	pf := &PathFinder{
		cache:        cache,
		priceCalc:    priceCalc, // 注入依赖
		maxHops:      3,
		adj:          make(map[string]map[string]bool),
		poolMap:      make(map[string]map[string][]*types.Pool),
		liquidityMap: make(map[string]map[string]*big.Int),
	}

	// TODO: 在这里启动一个 goroutine 来定期调用 pf.RefreshGraph(context.Background())
	// go pf.runGraphRefresher(context.Background())

	return pf
}

// 4. 新增: 图刷新方法
func (pf *PathFinder) RefreshGraph(ctx context.Context) error {
	log.Println("PathFinder: Refreshing graph from cache...")
	allPools, err := pf.cache.GetAllPools(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pools for graph refresh: %v", err)
	}

	// 构建新的图
	adj := make(map[string]map[string]bool)
	poolMap := make(map[string]map[string][]*types.Pool)
	liquidityMap := make(map[string]map[string]*big.Int)

	for _, pool := range allPools {
		t0 := strings.ToLower(pool.Token0.Address)
		t1 := strings.ToLower(pool.Token1.Address)

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
		if liquidityMap[t0] == nil {
			liquidityMap[t0] = make(map[string]*big.Int)
		}
		if liquidityMap[t1] == nil {
			liquidityMap[t1] = make(map[string]*big.Int)
		}

		adj[t0][t1] = true
		adj[t1][t0] = true
		poolMap[t0][t1] = append(poolMap[t0][t1], pool)
		poolMap[t1][t0] = append(poolMap[t1][t0], pool)

		poolLiquidity := new(big.Int).Mul(pool.Reserve0, pool.Reserve1)
		if existing, exists := liquidityMap[t0][t1]; exists {
			liquidityMap[t0][t1] = new(big.Int).Add(existing, poolLiquidity)
			liquidityMap[t1][t0] = new(big.Int).Add(existing, poolLiquidity)
		} else {
			liquidityMap[t0][t1] = poolLiquidity
			liquidityMap[t1][t0] = poolLiquidity
		}
	}

	// 5. 线程安全地替换旧的图
	pf.graphLock.Lock()
	pf.adj = adj
	pf.poolMap = poolMap
	pf.liquidityMap = liquidityMap
	pf.graphLock.Unlock()

	log.Printf("PathFinder: Graph refreshed, %d pools loaded.", len(allPools))
	return nil
}

// --- 优先队列的实现 (放在 path_finder.go 文件的末尾) ---

// pathState 存储在优先队列中的状态
type pathState struct {
	path      []*types.Pool // 到达此点的路径（由Pool组成）
	amountOut *big.Int      // 到达此点时拥有的token数量
	lastToken string        // 此路径的最后一个token
	index     int           // 在 heap 中的索引
}

// priorityQueue 实现了 heap.Interface
type priorityQueue []*pathState

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// 我们希望是最大堆 (Max-Heap)，所以按 amountOut 降序排列
	return pq[i].amountOut.Cmp(pq[j].amountOut) > 0
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*pathState)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // 避免内存泄漏
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// --- 重写 FindBestPaths ---

// FindBestPaths 寻找最优报价路径
func (pf *PathFinder) FindBestPaths(ctx context.Context, tokenIn, tokenOut string, amountIn *big.Int, maxHops, maxPaths int) ([][]*types.Pool, error) {
	if maxHops <= 0 {
		maxHops = pf.maxHops
	}

	normalizedTokenIn := strings.ToLower(tokenIn)
	normalizedTokenOut := strings.ToLower(tokenOut)

	log.Printf("PathFinder: Searching best paths from %s to %s (amountIn: %s, maxHops: %d, maxPaths: %d)",
		normalizedTokenIn, normalizedTokenOut, amountIn.String(), maxHops, maxPaths)

	// 1. 从缓存的图中读取数据 (使用读锁)
	pf.graphLock.RLock()
	defer pf.graphLock.RUnlock()

	if pf.adj[normalizedTokenIn] == nil {
		log.Printf("PathFinder: TokenIn %s not found in graph", normalizedTokenIn)
		return [][]*types.Pool{}, nil
	}
	if pf.adj[normalizedTokenOut] == nil {
		log.Printf("PathFinder: TokenOut %s not found in graph", normalizedTokenOut)
		return [][]*types.Pool{}, nil
	}

	var bestPaths [][]*types.Pool

	// 2. 初始化Dijkstra
	// 优先队列，按 amountOut 排序（最大堆）
	pq := make(priorityQueue, 0)
	heap.Init(&pq)

	// bestAmountPerToken 记录到达某个token的最高输出量，用于剪枝
	bestAmountPerToken := make(map[string]*big.Int)

	// 3. 将所有第一跳(hop)的路径加入队列
	// 遍历 tokenIn 的所有邻居
	for neighborToken := range pf.adj[normalizedTokenIn] {
		// 遍历 tokenIn 和 neighborToken 之间的所有池子
		for _, pool := range pf.poolMap[normalizedTokenIn][neighborToken] {
			// 模拟交易，计算第一跳的输出
			hopAmountOut, err := pf.priceCalc.CalculateOutput(pool, amountIn, normalizedTokenIn) // <--- 修正后
			if err != nil || hopAmountOut.Cmp(big.NewInt(0)) <= 0 {
				continue // 交易无效或无输出
			}

			newState := &pathState{
				path:      []*types.Pool{pool},
				amountOut: hopAmountOut,
				lastToken: neighborToken,
			}
			heap.Push(&pq, newState)

			if bestAmount, ok := bestAmountPerToken[neighborToken]; !ok || hopAmountOut.Cmp(bestAmount) > 0 {
				bestAmountPerToken[neighborToken] = hopAmountOut
			}
		}
	}

	// 4. 开始Dijkstra搜索
	for pq.Len() > 0 && len(bestPaths) < maxPaths {
		// 弹出当前具有最大 amountOut 的路径
		currentState := heap.Pop(&pq).(*pathState)

		// 检查是否是更优路径（剪枝）
		// 如果我们之前通过更短的路径（或相同长度）找到了到达此token的更优报价，则跳过
		if bestAmount, ok := bestAmountPerToken[currentState.lastToken]; ok {
			if currentState.amountOut.Cmp(bestAmount) < 0 {
				continue
			}
		}

		// 检查是否到达终点
		if currentState.lastToken == normalizedTokenOut {
			bestPaths = append(bestPaths, currentState.path)
			// 找到一条路径，继续搜索，直到满足 maxPaths
			continue
		}

		// 检查是否超过 maxHops
		if len(currentState.path) >= maxHops {
			continue
		}

		// 5. 探索邻居（下一跳）
		currentHopToken := currentState.lastToken
		currentHopAmountIn := currentState.amountOut

		for nextHopToken := range pf.adj[currentHopToken] {
			// 避免环路 (简单检查)
			if pf.pathContainsToken(currentState.path, nextHopToken) {
				continue
			}

			// 遍历 currentHopToken 和 nextHopToken 之间的所有池子
			for _, pool := range pf.poolMap[currentHopToken][nextHopToken] {

				// 模拟交易
				nextHopAmountOut, err := pf.priceCalc.CalculateOutput(pool, currentHopAmountIn, currentHopToken) // <--- 修正后
				if err != nil || nextHopAmountOut.Cmp(big.NewInt(0)) <= 0 {
					continue
				}

				// 检查这是否是一条通往 nextHopToken 的更优路径
				if bestAmount, ok := bestAmountPerToken[nextHopToken]; !ok || nextHopAmountOut.Cmp(bestAmount) > 0 {
					bestAmountPerToken[nextHopToken] = nextHopAmountOut

					// 创建新路径
					newPath := make([]*types.Pool, len(currentState.path)+1)
					copy(newPath, currentState.path)
					newPath[len(newPath)-1] = pool

					newState := &pathState{
						path:      newPath,
						amountOut: nextHopAmountOut,
						lastToken: nextHopToken,
					}
					heap.Push(&pq, newState)
				}
			}
		}
	}

	log.Printf("PathFinder: Found %d best paths.", len(bestPaths))
	return bestPaths, nil
}

// 辅助函数：检查路径是否已包含某个token (避免环路)
func (pf *PathFinder) pathContainsToken(path []*types.Pool, token string) bool {
	// 简单的环路检查：检查新token是否已在路径中（作为池子的任一端）
	// 注意: tokenIn 已经在之前的 hop 中了
	for _, pool := range path {
		if strings.ToLower(pool.Token0.Address) == token || strings.ToLower(pool.Token1.Address) == token {
			return true
		}
	}
	return false
}
