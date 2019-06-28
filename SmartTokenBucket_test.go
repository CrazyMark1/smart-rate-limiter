package limit

import (
	"testing"
	"time"
)

func TestTokenBucket_GetToken(t *testing.T) {
	tokenBucket := NewTokenBucket(10, 500, 80, 80)
	tokenBucket.GetToken()
}

func TestTokenBucket_TryGetToken(t *testing.T) {
	tokenBucket := NewTokenBucket(10, 500, 80, 80)
	tokenBucket.TryGetToken(100 * time.Millisecond)
}
