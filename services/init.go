package services

import "sync/atomic"

// services 包初始化语句

// services 包内全局的发现消息序号，为每个发现消息分配唯一序号
var globalDiscoverySeq atomic.Uint64