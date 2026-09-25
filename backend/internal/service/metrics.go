package service

import "sync/atomic"

// LLMErrors counts failed upstream calls, including calls recovered by a demo fallback.
var LLMErrors atomic.Int64
