package services

// 交换信息等候室模块
// UDP 组播和 TCP 连接传输过来的交换信息会集中存放在这里，等待转发

import (
	"container/heap"
	"sync"
	"time"
	"errors"

	"github.com/somebottle/localsend-switch/constants"
	"github.com/somebottle/localsend-switch/entities"
	"github.com/somebottle/localsend-switch/utils"
)

// TTL 堆元素
type TTLHeapItem struct {
	// 发现信息唯一 ID
	id string
	// 过期时间
	expireAt time.Time
}

// TTL 堆 (小根堆，越早过期的在前面)
type TTLHeap []*TTLHeapItem

// 实现 heap 接口

func (th *TTLHeap) Len() int {
	return len(*th)
}

func (th *TTLHeap) Less(i, j int) bool {
	return (*th)[i].expireAt.Before((*th)[j].expireAt)
}

func (th *TTLHeap) Swap(i, j int) {
	(*th)[i], (*th)[j] = (*th)[j], (*th)[i]
}

func (th *TTLHeap) Push(x any) {
	*th = append(*th, x.(*TTLHeapItem))
}

func (th *TTLHeap) Pop() any {
	item := (*th)[len(*th)-1]
	*th = (*th)[:len(*th)-1]
	return item
}

// SwitchLounge 交换信息等候室
type SwitchLounge struct {
	// heap, map 更新锁
	mutex sync.Mutex
	// 存储已经转发了的发现信息 ID，防止传播路径有环，重复转发
	forwardedIds map[string]bool
	// 关闭信号，让相应协程退出
	closeSignal chan struct{}
	// 标记是否关闭
	closed bool
	// 维护发现包 ID 的过期时间的堆
	ttlHeap *TTLHeap
	// 交换数据等候区，这些数据会被转发到其他节点
	// 每个数据不会被发向其来源节点
	lounge chan *entities.SwitchMessage
}

// NewSwitchLounge 创建一个新的交换信息等候室
func NewSwitchLounge() *SwitchLounge {
	ttlHeap := &TTLHeap{}
	heap.Init(ttlHeap)
	switchLounge := SwitchLounge{
		closeSignal: make(chan struct{}),
		forwardedIds:  make(map[string]bool),
		ttlHeap:     ttlHeap,
		lounge:      make(chan *entities.SwitchMessage, constants.SWITCH_LOUNGE_SIZE),
	}
	// 过期 ID 清理协程
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-switchLounge.closeSignal:
				// 退出协程
				return
			case <-ticker.C:
				now := time.Now()
				switchLounge.mutex.Lock()
				for {
					if ttlHeap.Len() == 0 {
						// 堆空
						break
					}
					// 把过期的 ID 都清理掉，直至堆顶未过期
					item := (*ttlHeap)[0]
					if item.expireAt.After(now) {
						break
					}
					heap.Pop(ttlHeap)
					// 删除 ID 记录
					delete(switchLounge.forwardedIds, item.id)
				}
				switchLounge.mutex.Unlock()
			}
		}
	}()
	return &switchLounge
}

// Write 将交换信息写入等候室，等待转发
// 重复的发现包会被忽略
func (sl *SwitchLounge) Write(msg *entities.SwitchMessage) error {
	discoveryId := utils.GetDiscoveryId(msg)

	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	if sl.closed {
		// 已关闭，忽略写入
		return errors.New("Switch lounge is closed")
	}
	// 检查是否已经转发过该发现包
	if _, exists := sl.forwardedIds[discoveryId]; exists {
		// 已经转发过，忽略
		return nil
	}
	// 如果条目过多，放弃写入
	if len(sl.forwardedIds) >= constants.SWITCH_ID_CACHE_MAX_ENTRIES {
		return errors.New("Switch lounge relayed ID cache is full")
	}

	// 没有转发过，则把交换信息写入等候通道
	select {
	case sl.lounge <- msg:
		// 记录该发现包 ID，防止重复转发
		sl.forwardedIds[discoveryId] = true
		// 加入堆中
		heap.Push(sl.ttlHeap, &TTLHeapItem{
			id:        discoveryId,
			expireAt: time.Now().Add(constants.SWITCH_ID_CACHE_LIFETIME * time.Second),
		})
	default:
		// 等候通道已满，忽略写入
		return errors.New("Switch lounge is full")
	}
	return nil
}

// Read 返回一个通道，从中可以读取到等待转发的交换信息
func (sl *SwitchLounge) Read() <-chan *entities.SwitchMessage {
	return sl.lounge
}

// Close 关闭交换信息等候室，释放资源
func (sl *SwitchLounge) Close() {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	if sl.closed {
		// 已关闭，忽略
		return
	}
	// 这里也会让相应协程退出
	sl.closed = true
	close(sl.closeSignal)
	close(sl.lounge)
}
