package gomodule_test

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cameronelliott/redislock"
	redigoclient "github.com/cameronelliott/redislock/examples/redigo/redisclient"
	"github.com/gomodule/redigo/redis"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const lockKey = "__bsm_redislock_unit_test__"

var _ = Describe("Client", func() {
	var subject *redislock.Client

	BeforeEach(func() {
		subject = redislock.New(redisClient)
	})

	AfterEach(func() {
		conn := redisPool.Get()
		defer conn.Close()
		_, err := redis.Int64(conn.Do("DEL", lockKey))
		Expect(err).To(Succeed())
	})

	It("should obtain once with TTL", func() {
		lock1, err := subject.Obtain(lockKey, time.Hour, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(lock1.Token()).To(HaveLen(22))
		Expect(lock1.TTL()).To(BeNumerically("~", time.Hour, time.Second))
		defer func() { _ = lock1.Release() }()

		_, err = subject.Obtain(lockKey, time.Hour, nil)
		Expect(err).To(Equal(redislock.ErrNotObtained))
		Expect(lock1.Release()).To(Succeed())

		lock2, err := subject.Obtain(lockKey, time.Minute, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(lock2.TTL()).To(BeNumerically("~", time.Minute, time.Second))
		Expect(lock2.Release()).To(Succeed())
	})

	It("should obtain through short-cut", func() {
		lock, err := redislock.Obtain(redisClient, lockKey, time.Hour, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(lock.Release()).To(Succeed())
	})

	It("should support custom metadata", func() {
		lock, err := redislock.Obtain(redisClient, lockKey, time.Hour, &redislock.Options{Metadata: "my-data"})
		Expect(err).NotTo(HaveOccurred())
		Expect(lock.Metadata()).To(Equal("my-data"))
		Expect(lock.Release()).To(Succeed())
	})

	It("should refresh", func() {
		lock, err := redislock.Obtain(redisClient, lockKey, time.Minute, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(lock.TTL()).To(BeNumerically("~", time.Minute, time.Second))
		Expect(lock.Refresh(time.Hour, nil)).To(Succeed())
		Expect(lock.TTL()).To(BeNumerically("~", time.Hour, time.Second))
		Expect(lock.Release()).To(Succeed())
	})

	It("should fail to release if expired", func() {
		lock, err := redislock.Obtain(redisClient, lockKey, time.Millisecond, nil)
		Expect(err).NotTo(HaveOccurred())
		time.Sleep(5 * time.Millisecond)
		Expect(lock.Release()).To(MatchError(redislock.ErrLockNotHeld))
	})

	It("should fail to release if obtained by someone else", func() {
		lock, err := redislock.Obtain(redisClient, lockKey, time.Minute, nil)
		Expect(err).NotTo(HaveOccurred())

		conn := redisPool.Get()
		defer conn.Close()
		_, err = conn.Do("SET", lockKey, "ABCD")
		Expect(err).NotTo(HaveOccurred())
		Expect(lock.Release()).To(MatchError(redislock.ErrLockNotHeld))
	})

	It("should fail to refresh if expired", func() {
		lock, err := redislock.Obtain(redisClient, lockKey, time.Millisecond, nil)
		Expect(err).NotTo(HaveOccurred())
		time.Sleep(5 * time.Millisecond)
		Expect(lock.Refresh(time.Hour, nil)).To(MatchError(redislock.ErrNotObtained))
	})

	It("should retry if enabled", func() {
		// retry, succeed
		conn := redisPool.Get()
		defer conn.Close()
		_, err := conn.Do("SET", lockKey, "ABCD")
		Expect(err).NotTo(HaveOccurred())
		//20 millisecond
		_, err = conn.Do("PEXPIRE", lockKey, 20)
		Expect(err).NotTo(HaveOccurred())
		lock, err := redislock.Obtain(redisClient, lockKey, time.Hour, &redislock.Options{
			RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(100*time.Millisecond), 3),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(lock.Release()).To(Succeed())

		// no retry, fail
		_, err = conn.Do("SET", lockKey, "ABCD")
		Expect(err).NotTo(HaveOccurred())
		_, err = conn.Do("PEXPIRE", lockKey, 50)
		Expect(err).NotTo(HaveOccurred())

		_, err = redislock.Obtain(redisClient, lockKey, time.Hour, nil)
		Expect(err).To(MatchError(redislock.ErrNotObtained))

		// // retry 2x, give up & fail
		_, err = conn.Do("SET", lockKey, "ABCD")
		Expect(err).NotTo(HaveOccurred())
		_, err = conn.Do("PEXPIRE", lockKey, 50)
		Expect(err).NotTo(HaveOccurred())

		_, err = redislock.Obtain(redisClient, lockKey, time.Hour, &redislock.Options{
			RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(time.Millisecond), 2),
		})
		Expect(err).To(MatchError(redislock.ErrNotObtained))

		// // retry 3x, pass
		_, err = conn.Do("SET", lockKey, "ABCD")
		Expect(err).NotTo(HaveOccurred())
		_, err = conn.Do("PEXPIRE", lockKey, 50)
		Expect(err).NotTo(HaveOccurred())

		lock, err = redislock.Obtain(redisClient, lockKey, 2*time.Hour, &redislock.Options{
			RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(20*time.Millisecond), 3),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(lock.TTL()).To(BeNumerically("~", 2*time.Hour, time.Second))
		Expect(lock.Release()).To(Succeed())
	})

	It("should prevent multiple locks (fuzzing)", func() {
		numLocks := int32(0)
		wg := new(sync.WaitGroup)
		for i := 0; i < 1000; i++ {
			wg.Add(1)

			go func() {
				defer GinkgoRecover()
				defer wg.Done()

				wait := rand.Int63n(int64(50 * time.Millisecond))
				time.Sleep(time.Duration(wait))

				_, err := subject.Obtain(lockKey, time.Minute, nil)
				if err == redislock.ErrNotObtained {
					return
				}
				Expect(err).NotTo(HaveOccurred())
				atomic.AddInt32(&numLocks, 1)
			}()
		}
		wg.Wait()
		Expect(numLocks).To(Equal(int32(1)))
	})

})

var _ = Describe("RetryStrategy", func() {
	It("should support no-retry", func() {
		subject := redislock.NoRetry()
		Expect(subject.NextBackoff()).To(Equal(time.Duration(0)))
	})

	It("should support linear backoff", func() {
		subject := redislock.LinearBackoff(time.Second)
		Expect(subject.NextBackoff()).To(Equal(time.Second))
		Expect(subject.NextBackoff()).To(Equal(time.Second))
	})

	It("should support limits", func() {
		subject := redislock.LimitRetry(redislock.LinearBackoff(time.Second), 2)
		Expect(subject.NextBackoff()).To(Equal(time.Second))
		Expect(subject.NextBackoff()).To(Equal(time.Second))
		Expect(subject.NextBackoff()).To(Equal(time.Duration(0)))
	})

	It("should support exponential backoff", func() {
		subject := redislock.ExponentialBackoff(10*time.Millisecond, 300*time.Millisecond)
		Expect(subject.NextBackoff()).To(Equal(10 * time.Millisecond))
		Expect(subject.NextBackoff()).To(Equal(10 * time.Millisecond))
		Expect(subject.NextBackoff()).To(Equal(16 * time.Millisecond))
		Expect(subject.NextBackoff()).To(Equal(32 * time.Millisecond))
		Expect(subject.NextBackoff()).To(Equal(64 * time.Millisecond))
		Expect(subject.NextBackoff()).To(Equal(128 * time.Millisecond))
		Expect(subject.NextBackoff()).To(Equal(256 * time.Millisecond))
		Expect(subject.NextBackoff()).To(Equal(300 * time.Millisecond))
		Expect(subject.NextBackoff()).To(Equal(300 * time.Millisecond))
		Expect(subject.NextBackoff()).To(Equal(300 * time.Millisecond))
	})
})

// --------------------------------------------------------------------

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "redisclient")
}

var redisPool *redis.Pool
var redisClient *redigoclient.RedisLockClient
var _ = BeforeSuite(func() {
	redisPool = &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", ":6379",
				redis.DialDatabase(1))
		},
	}
	conn := redisPool.Get()
	defer conn.Close()
	Expect(conn.Err()).To(Succeed())
	redisClient = redigoclient.NewRedisLockClient(redisPool)
})

var _ = AfterSuite(func() {
	Expect(redisPool.Close()).To(Succeed())
})
