# redislock

[![Build Status](https://travis-ci.com/dineshgowda24/redislock.svg)](https://travis-ci.com/dineshgowda24/redislock)
[![GoDoc](https://godoc.org/github.com/dineshgowda24/redislock?status.png)](http://godoc.org/github.com/bsm/redislock)
[![Go Report Card](https://goreportcard.com/badge/github.com/bsm/redislock)](https://goreportcard.com/report/github.com/bsm/redislock)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Simplified distributed locking implementation using [Redis](http://redis.io/topics/distlock).
For more information, please see examples.

## Examples

```go
import (
  "fmt"
  "time"

  "github.com/bsm/redislock"
  "github.com/go-redis/redis/v7"
)

func main() {
	// Connect to redis.
	client := &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", ":6379",
				redis.DialDatabase(1))
		},
	}

	// Create a new lock client.
	locker := redislock.New(client)

	// Try to obtain lock.
	lock, err := locker.Obtain("my-key", 100*time.Millisecond, nil)
	if err == redislock.ErrNotObtained {
		fmt.Println("Could not obtain lock!")
	} else if err != nil {
		log.Fatalln(err)
	}

	// Don't forget to defer Release.
	defer lock.Release()
	fmt.Println("I have a lock!")

	// Sleep and check the remaining TTL.
	time.Sleep(50 * time.Millisecond)
	if ttl, err := lock.TTL(); err != nil {
		log.Fatalln(err)
	} else if ttl > 0 {
		fmt.Println("Yay, I still have my lock!")
	}

	// Extend my lock.
	if err := lock.Refresh(100*time.Millisecond, nil); err != nil {
		log.Fatalln(err)
	}

	// Sleep a little longer, then check.
	time.Sleep(100 * time.Millisecond)
	if ttl, err := lock.TTL(); err != nil {
		log.Fatalln(err)
	} else if ttl == 0 {
		fmt.Println("Now, my lock has expired!")
	}

	// Output:
	// I have a lock!
	// Yay, I still have my lock!
	// Now, my lock has expired!

}
```

## Documentation

Full documentation is available on [GoDoc](http://godoc.org/github.com/bsm/redislock)
