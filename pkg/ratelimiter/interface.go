package ratelimiter

type Interface interface {
	Allow(key string, kind string) bool
}