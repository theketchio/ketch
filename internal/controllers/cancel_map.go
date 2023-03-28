package controllers

import (
	"context"
	"sync"
)

// CancelMap helps AppReconciler tracking goroutines.
// There are two use cases:
//  1. AppReconciler starts a goroutine and forgets about it never trying to cancel it.
//     cleanupFunc is responsible for calling cancel() and removing all resources associated with it.
//  1. AppReconciler starts a new goroutine and cancels the previous one.
//     cleanupFunc of the previous goroutine must not do any cleanup.
type CancelMap struct {
	sync.Mutex
	generation       int64
	cancelMap        map[string]context.CancelFunc
	cancelGeneration map[string]int64
}

// cleanupFunc is a wrapper for context.CancelFunc,
// when called it additionally removes this cancel function from being tracked by CancelMap.
type cleanupFunc func()

func NewCancelMap() *CancelMap {
	return &CancelMap{
		generation:       1,
		cancelMap:        map[string]context.CancelFunc{},
		cancelGeneration: map[string]int64{},
	}
}

func (cm *CancelMap) replaceAndCancelPrevious(name string, cancel context.CancelFunc) cleanupFunc {
	cm.Lock()
	defer cm.Unlock()

	g := cm.generation
	cm.generation += 1

	if c, ok := cm.cancelMap[name]; ok {
		c()
	}

	cm.cancelMap[name] = cancel
	cm.cancelGeneration[name] = g

	return func() {
		cm.Lock()
		defer cm.Unlock()

		cancel()

		if cm.cancelGeneration[name] != g {
			return
		}

		delete(cm.cancelMap, name)
		delete(cm.cancelGeneration, name)
	}
}
