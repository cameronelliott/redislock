# redislock

This package is originally by https://github.com/dineshgowda24  
Thank you to Dinesh for creating this.

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Simplified distributed locking implementation using [Redis](http://redis.io/topics/distlock).
For more information, please see examples.

This intro is written by Dinesh.

## Motivation

I came across a concurrency issue when multiple clients were accessing single redis instance. So I wanted a primitive locking solution, but redis did not have one implemented. So started looking for open source libraries and found [redislock](https://github.com/bsm/redislock) very well written and effective library. But it still did not solve my problem as I was using [redigo](https://github.com/gomodule/redigo) client but the package used [go-redis](https://github.com/go-redis/redis). Although `redigo` had [`redsync`](https://github.com/go-redsync/redsync), I wanted a much more simpler one and so with `redislock`.

## Features

 - Simple and easy to use interface.
 - Plug in any redis client of your choice by implementing the `RedisClient` interface.
 - Simple but effective locking for single redis instance.

## Examples

Check out examples in for [`redigo`](./examples/redigo) and [`go-redis`](./examples/goredis) clients.

## Documentation

Full documentation is available on [GoDoc](http://godoc.org/github.com/cameronelliott/redislock)

## Contribution

Feel free to send a PR.