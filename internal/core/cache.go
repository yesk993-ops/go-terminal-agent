package core

import "time"

type Cache interface {
	Get(key string) (string, bool)
	Set(key string, value string, ttl time.Duration)
	Delete(key string)
	Clear()
	Len() int
}
