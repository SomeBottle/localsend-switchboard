package services

// 存放本地 LocalSend 客户端信息的等候室
// 按理说就只有一个本地客户端，但为了扩展性，还是用 map 存储多个

import (
	"sync"
	"time"

	"github.com/somebottle/localsend-switch/constants"
	"github.com/somebottle/localsend-switch/entities"
)

type LocalClientInfoWithTTL struct {
	info     *entities.LocalSendClientInfo
	expireAt time.Time
}

// LocalClientLounge 存放本地 LocalSend 客户端信息
type LocalClientLounge struct {
	mutex           sync.Mutex                         // 保护 clientInfos 的并发访问
	clientInfos     map[uint16]*LocalClientInfoWithTTL // key: 本地客户端监听的端口
	closeSignal     chan struct{}                      // 关闭信号，让相应协程退出
	closed          bool                               // 标记是否关闭
}

// NewLocalClientLounge 创建一个新的本地客户端信息等候室
func NewLocalClientLounge() *LocalClientLounge {
	lcl := LocalClientLounge{
		clientInfos: make(map[uint16]*LocalClientInfoWithTTL),
		closeSignal: make(chan struct{}),
		closed:      false,
	}
	// 定时清理过期客户端信息的协程
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				lcl.mutex.Lock()
				now := time.Now()
				// 本地其实不会有太多 LocalSend 客户端，所以直接遍历即可，不需要维护堆
				for port, infoWithTTL := range lcl.clientInfos {
					if infoWithTTL.expireAt.Before(now) {
						delete(lcl.clientInfos, port)
					}
				}
				lcl.mutex.Unlock()
			case <-lcl.closeSignal:
				return
			}
		}
	}()
	return &lcl
}

// Add 添加或更新本地客户端信息，如果已经存在则更新其过期时间
func (lcl *LocalClientLounge) Add(info *entities.LocalSendClientInfo) {
	lcl.mutex.Lock()
	defer lcl.mutex.Unlock()
	if lcl.closed {
		return
	}
	lcl.clientInfos[info.Port] = &LocalClientInfoWithTTL{
		info:     info,
		expireAt: time.Now().Add(constants.LOCAL_CLIENT_INFO_CACHE_LIFETIME * time.Second), // 更新信息有效期
	}
}

// SyncGet 获取所有现有本地客户端信息，以通道形式返回
//
// 注：在读取完毕前会锁住等候室，防止并发修改，因为本地客户端其实往往只有 1 个，这通常不会是很大问题
func (lcl *LocalClientLounge) SyncGet() <-chan *entities.LocalSendClientInfo {
	outChan := make(chan *entities.LocalSendClientInfo)
	go func() {
		lcl.mutex.Lock()
		defer close(outChan)
		defer lcl.mutex.Unlock()
		if lcl.closed {
			return
		}
		for _, infoWithTTL := range lcl.clientInfos {
			outChan <- infoWithTTL.info
		}
	}()
	return outChan
}

// Close 关闭本地客户端信息等候室
func (lcl *LocalClientLounge) Close() {
	lcl.mutex.Lock()
	defer lcl.mutex.Unlock()
	if lcl.closed {
		return
	}
	close(lcl.closeSignal)
	lcl.closed = true
}
