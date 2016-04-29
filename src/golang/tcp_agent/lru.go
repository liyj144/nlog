package tcp_agent

import "container/list"
import "sync"
import "time"
import "fmt"

//10 minutes
const ExpireTimeInMilliSecond = 15*60*1000

//60 seconds
const WorkerWakeupInSecond = 10*60

// TokenCache is an  modified version of LRU cache. It is not safe for concurrent access.
// Unlike lrucache we disable move to front when access specific itemi, which make it safe for concurrent access
// we assume that uid-token is one to many relation
type TokenCache struct {
        s  *Server

	// MaxEntries is the maximum number of cache entries before
	// an item is evicted. Zero means no limit.
	MaxEntries int

	// OnEvicted optionally specificies a callback function to be
	// executed when an entry is purged from the cache.
	OnEvicted func(key string, value uint64)

        now   int64

        rwm   sync.RWMutex
	ll    *list.List
	cache map[string]*list.Element
}


type entry struct {
	token   string
	uid     uint64
        expire_time int64
}

// New creates a new TokenCache.
// If maxEntries is zero, the cache has no limit and it's assumed
// that eviction is done by the caller.
func NewTokenCache(s *Server) (tc *TokenCache) {
	tc = &TokenCache{
		s: s,
		ll:         list.New(),
		cache:      make(map[string]*list.Element,10000),
	}
        //start cleanup worker here
        go tc.RemoveExpireItemsWorker()
        return tc
}

// Add adds a value to the cache.
func (c *TokenCache) Add(token string, uid uint64) {
        // cache should not be nil here
	//if c.cache == nil {
	//	c.cache = make(map[string]*list.Element)
	//	c.ll = list.New()
	//}
	c.rwm.Lock()
	if ee, ok := c.cache[token]; ok {
		//c.ll.MoveToFront(ee)
		ee.Value.(*entry).uid = uid
                c.rwm.Unlock()
		return
	}
	ele := c.ll.PushFront(&entry{token:token,
                                uid: uid,
                                expire_time: c.now + ExpireTimeInMilliSecond })
	c.cache[token] = ele
	c.rwm.Unlock()
}

// Get looks up a key's value from the cache.
func (c *TokenCache) Get(token string,time_now int64) (uid uint64, ok bool) {
        c.now = time_now
        //c.s.logf("lru:get %s",token)
        c.rwm.RLock()
	if ele, hit := c.cache[token]; hit {
		//c.ll.MoveToFront(ele)
                c.rwm.RUnlock()
                //c.s.logf("lru:uid %s",ele.Value.(*entry).uid)
		return ele.Value.(*entry).uid, true
	}
	c.rwm.RUnlock()
	return 0,false
}

// Remove removes the provided key from the cache.
func (c *TokenCache) Remove(token string) {
	c.rwm.Lock()
	if ele, hit := c.cache[token]; hit {
		c.removeElement(ele)
	}
	c.rwm.Unlock()
}

func (c *TokenCache) dumpItems() {
        for k,v := range c.cache {
                fmt.Printf("dump: %s,%d\n",k,v.Value.(*entry).expire_time)

        }
        fmt.Println("list")
        for e := c.ll.Front(); e != nil; e = e.Next() {
                fmt.Printf("dump: %s,%d\n",e.Value.(*entry).token,e.Value.(*entry).expire_time)
        }
}

//TODO refine purge logic here
// RemoveExpireItemsWorker removes expire items from the cache.
func (c *TokenCache) RemoveExpireItemsWorker() {
        for {
                //c.s.logf("lru:wakeup ")
                c.rwm.Lock()
	        ele := c.ll.Back()
	        for ele != nil {
                        if ele.Value.(*entry).expire_time > c.now { 
                                //c.dumpItems()
                                break
                        }
                        //c.s.logf("lru:remove %d, %d",c.now,ele.Value.(*entry).expire_time)
	        	c.removeElement(ele)
                        ele = c.ll.Back()
	        }
                c.rwm.Unlock()
                time.Sleep(WorkerWakeupInSecond * time.Second)
        }
}

func (c *TokenCache) removeElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*entry)
        //c.s.logf("lru:remove %s",kv.token)
	delete(c.cache, kv.token)
	if c.OnEvicted != nil {
		c.OnEvicted(kv.token, kv.uid)
	}
}

// Len returns the number of items in the cache.
func (c *TokenCache) Len() int {
	return c.ll.Len()
}

