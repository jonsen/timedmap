package timedmap

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNew(t *testing.T) {
	tm := New(5 * time.Second).(*timedMap)
	assert.True(t, tm.cleanupRunning)
}

func TestSet(t *testing.T) {
	tm := New(5 * time.Second).(*timedMap)
	const lifetime = 1 * time.Second
	tm.Set("test", 1, lifetime)
	setTime := time.Now()

	_vw, _ := tm.m.Load("test")
	vw := _vw.(*valueWrapper)
	assert.Equal(t, 1, vw.val)
	assert.InDelta(t,
		setTime.Add(lifetime).UnixNano(),
		vw.exp.UnixNano(),
		float64(1*time.Millisecond))
	assert.EqualValues(t, 1, tm.size)
}

func TestGetValue(t *testing.T) {
	// Test Get + Expire Get + Callback
	{
		cb := new(cbMock)
		cb.On("Cb").Return()

		tm := New(5 * time.Second)
		tm.Set("test", 1, 1*time.Second, cb.Cb)

		v := tm.GetValue("test")
		assert.Equal(t, 1, v)

		time.Sleep(1 * time.Second)

		v = tm.GetValue("test")
		assert.Nil(t, v)

		cb.AssertCalled(t, "Cb")
		assert.Equal(t, 1, cb.TestData().Get("v").Int())
	}

	// Test Expire Cleanup + Callback
	{
		cb := new(cbMock)
		cb.On("Cb").Return()

		tm := New(500 * time.Millisecond).(*timedMap)
		tm.Set("test", 1, 0, cb.Cb)

		time.Sleep(1 * time.Second)

		_, ok := tm.m.Load("test")
		assert.False(t, ok)

		cb.AssertCalled(t, "Cb")
		assert.Equal(t, 1, cb.TestData().Get("v").Int())
	}

	// Test Size on double Set
	{
		tm := New(500 * time.Millisecond).(*timedMap)

		tm.Set("test", 1, 1*time.Second)
		assert.EqualValues(t, 1, tm.size)

		tm.Set("test2", 1, 1*time.Second)
		assert.EqualValues(t, 2, tm.size)

		tm.Set("test", 3, 1*time.Second)
		assert.EqualValues(t, 2, tm.size)
	}
}

func TestGetExpires(t *testing.T) {
	tm := New(5 * time.Second)
	const lifetime = 1 * time.Second
	tm.Set("test", 1, lifetime)
	setTime := time.Now()

	exp, err := tm.GetExpires("test")
	assert.Nil(t, err)
	assert.InDelta(t,
		setTime.Add(lifetime).UnixNano(),
		exp.UnixNano(),
		float64(1*time.Millisecond))

	time.Sleep(1 * time.Second)

	exp, err = tm.GetExpires("test")
	assert.EqualError(t, err, ErrKeyNotFound.Error())
}

func TestSetExpires(t *testing.T) {
	tm := New(5 * time.Second).(*timedMap)
	const lifetime = 1 * time.Second
	tm.Set("test", 1, lifetime)

	time.Sleep(500 * time.Millisecond)
	setTime := time.Now()

	err := tm.SetExpire("test", lifetime)
	assert.Nil(t, err)

	_vw, _ := tm.m.Load("test")
	vw := _vw.(*valueWrapper)
	assert.InDelta(t,
		setTime.Add(lifetime).UnixNano(),
		vw.exp.UnixNano(),
		float64(1*time.Millisecond))

	err = tm.SetExpire("nonexistent", 0)
	assert.EqualError(t, err, ErrKeyNotFound.Error())
}

func TestRefresh(t *testing.T) {
	tm := New(5 * time.Second).(*timedMap)
	const lifetime = 1 * time.Second
	tm.Set("test", 1, lifetime)

	time.Sleep(500 * time.Millisecond)

	_vw, _ := tm.m.Load("test")
	vw := _vw.(*valueWrapper)
	expBefore := vw.exp

	err := tm.Refresh("test", lifetime)
	assert.Nil(t, err)

	_vw, _ = tm.m.Load("test")
	vw = _vw.(*valueWrapper)
	assert.EqualValues(t, expBefore.Add(lifetime), vw.exp)

	err = tm.Refresh("nonexistent", 0)
	assert.EqualError(t, err, ErrKeyNotFound.Error())
}

func TestContains(t *testing.T) {
	tm := New(5 * time.Second)
	tm.Set("test", 1, 1*time.Second)

	assert.True(t, tm.Contains("test"))
	time.Sleep(1 * time.Second)
	assert.False(t, tm.Contains("test"))
}

func TestRemove(t *testing.T) {
	cb := new(cbMock)
	cb.On("Cb").Return()

	tm := New(5 * time.Second).(*timedMap)
	tm.Set("test", 1, 1*time.Second, cb.Cb)
	assert.EqualValues(t, 1, tm.size)

	tm.Remove("test")
	assert.EqualValues(t, 0, tm.size)

	cb.AssertCalled(t, "Cb")
	assert.Equal(t, 1, cb.TestData().Get("v").Int())
}

func TestFlush(t *testing.T) {
	cb := new(cbMock)
	cb.On("Cb").Return()

	tm := New(5 * time.Second).(*timedMap)

	tm.Set("test1", 1, 1*time.Second, cb.Cb)
	tm.Set("test2", 1, 1*time.Second, cb.Cb)
	tm.Set("test3", 1, 1*time.Second, cb.Cb)
	assert.EqualValues(t, 3, tm.size)

	tm.Flush()
	assert.EqualValues(t, 0, tm.size)

	_, ok := tm.m.Load("test1")
	assert.False(t, ok)
	_, ok = tm.m.Load("test2")
	assert.False(t, ok)
	_, ok = tm.m.Load("test3")
	assert.False(t, ok)

	cb.AssertNumberOfCalls(t, "Cb", 3)
}

func TestSize(t *testing.T) {
	tm := New(5 * time.Second).(*timedMap)
	assert.EqualValues(t, 0, tm.Size())

	tm.Set("test1", 1, 1*time.Second)
	assert.EqualValues(t, 1, tm.Size())

	tm.Set("test2", 1, 1*time.Second)
	assert.EqualValues(t, 2, tm.Size())

	tm.Set("test2", 2, 1*time.Second)
	assert.EqualValues(t, 2, tm.Size())

	tm.Set("test3", 1, 1*time.Second)
	assert.EqualValues(t, 3, tm.Size())
}

func TestStartCleaner(t *testing.T) {
	var exp time.Duration
	cleanupInterval := 500 * time.Millisecond
	tm := New(cleanupInterval).(*timedMap)

	tset := time.Now()
	tm.Set("test", 1, 0, func(value interface{}) {
		exp = time.Since(tset)
	})

	time.Sleep(1 * time.Second)
	assert.InDelta(t,
		cleanupInterval,
		exp,
		float64(100*time.Millisecond))

	cleanupInterval = 1 * time.Second
	tm.StartCleaner(1 * time.Second)
	tset = time.Now()
	tm.Set("test", 1, 0, func(value interface{}) {
		exp = time.Since(tset)
	})

	time.Sleep(1500 * time.Millisecond)
	assert.InDelta(t,
		cleanupInterval,
		exp,
		float64(100*time.Millisecond))
}

func TestStopCleaner(t *testing.T) {
	cb := new(cbMock)
	cb.On("Cb").Return()

	tm := New(500 * time.Millisecond).(*timedMap)
	tm.Set("test1", 1, 0, cb.Cb)

	tm.StopCleaner()

	time.Sleep(1 * time.Second)

	cb.AssertNotCalled(t, "Cb")
}

//////////////

type cbMock struct {
	mock.Mock
}

func (cb *cbMock) Cb(v interface{}) {
	cb.TestData().Set("v", v)
	cb.Called()
}
