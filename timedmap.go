package timedmap

import (
	"sync"
	"sync/atomic"
	"time"
)

type Callback func(value interface{})

type Map interface {
	Set(key, value interface{}, expiresAfter time.Duration, cb ...Callback)
	GetValue(key interface{}) interface{}
	GetExpires(key interface{}) (time.Time, error)
	SetExpire(key interface{}, d time.Duration) error
	Refresh(key interface{}, d time.Duration) error
	Contains(key interface{}) bool
	Remove(key interface{})
	Flush()
	Size() int
}

type TimedMap interface {
	Map

	StartCleaner(interval time.Duration)
	StopCleaner()
}

////////////////////
// IMPLEMENTATION

type timedMap struct {
	m              *sync.Map
	valuePool      *sync.Pool
	size           int64
	cleanupRunning bool
	cStopCleanup   chan struct{}
}

type valueWrapper struct {
	val interface{}
	exp time.Time
	cb  Callback
}

func New(cleanupInterval time.Duration) TimedMap {
	t := &timedMap{
		m: &sync.Map{},
		valuePool: &sync.Pool{
			New: func() interface{} {
				return &valueWrapper{}
			},
		},
		cStopCleanup: make(chan struct{}),
	}

	t.StartCleaner(cleanupInterval)

	return t
}

func (t *timedMap) Set(key, value interface{}, expiresAfter time.Duration, cb ...Callback) {
	vw, ok := t.get(key)
	if !ok {
		vw = t.valuePool.Get().(*valueWrapper)
		atomic.AddInt64(&t.size, 1)
	}

	vw.val = value
	vw.exp = time.Now().Add(expiresAfter)
	if len(cb) > 0 {
		vw.cb = cb[0]
	} else {
		vw.cb = nil
	}

	t.m.Store(key, vw)
}

func (t *timedMap) GetValue(key interface{}) (v interface{}) {
	vw, ok := t.get(key)
	if !ok {
		return
	}

	return vw.val
}

func (t *timedMap) GetExpires(key interface{}) (exp time.Time, err error) {
	vw, ok := t.get(key)
	if !ok {
		err = ErrKeyNotFound
		return
	}

	exp = vw.exp
	return
}

func (t *timedMap) SetExpire(key interface{}, d time.Duration) (err error) {
	vw, ok := t.get(key)
	if !ok {
		err = ErrKeyNotFound
		return
	}

	vw.exp = time.Now().Add(d)

	return
}

func (t *timedMap) Refresh(key interface{}, d time.Duration) (err error) {
	vw, ok := t.get(key)
	if !ok {
		err = ErrKeyNotFound
		return
	}

	vw.exp = vw.exp.Add(d)

	return
}

func (t *timedMap) Contains(key interface{}) (ok bool) {
	_, ok = t.get(key)
	return
}

func (t *timedMap) Remove(key interface{}) {
	t.remove(key, nil)
}

func (t *timedMap) Flush() {
	t.m.Range(func(key, value interface{}) bool {
		vw, ok := value.(*valueWrapper)
		if ok {
			t.remove(key, vw)
		}
		return true
	})
}

func (t *timedMap) Size() int {
	return int(t.size)
}

func (t *timedMap) StartCleaner(interval time.Duration) {
	if t.cleanupRunning {
		t.StopCleaner()
	}
	t.cleanupRunning = true
	go t.cleanupCycle(interval)
}

func (t *timedMap) StopCleaner() {
	t.cStopCleanup <- struct{}{}
}

func (t *timedMap) get(key interface{}) (vw *valueWrapper, ok bool) {
	_vw, ok := t.m.Load(key)
	if !ok {
		return
	}
	vw, ok = _vw.(*valueWrapper)
	if !ok {
		return
	}

	if time.Now().After(vw.exp) {
		t.remove(key, vw)
		return nil, false
	}

	return
}

func (t *timedMap) remove(key interface{}, vw *valueWrapper) {
	if vw == nil {
		var ok bool
		if vw, ok = t.get(key); !ok {
			return
		}
	}

	if vw.cb != nil {
		vw.cb(vw.val)
	}
	t.m.Delete(key)
	t.valuePool.Put(vw)
	atomic.AddInt64(&t.size, -1)
}

func (t *timedMap) cleanup() {
	t.m.Range(func(key, value interface{}) bool {
		t.get(key)
		return true
	})
}

func (t *timedMap) cleanupCycle(interval time.Duration) {
	ticker := time.NewTicker(interval)

	defer func() {
		t.cleanupRunning = false
	}()

	for {
		select {
		case <-ticker.C:
			go t.cleanup()
		case <-t.cStopCleanup:
			ticker.Stop()
			break
		}
	}

}
