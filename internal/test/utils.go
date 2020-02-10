package test

import (
	"math/rand"
	"time"

	"github.com/knl/pulley/internal/events"
)

const (
	DefaultRepository = "knl/pulley"
	ZeroSHA           = "0000000000000000000000000000000000000000"
)

const (
	letterBytes   = "abcdef0123456789"
	letterIdxBits = 4                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// RandSHA returns a random SHA
// This is an optimized version of a generator, taken from:
// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
func RandSHA() string {
	n := 40
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdxMax letters!
	for i, cache, remain := n-1, rand.Uint64(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Uint64(), letterIdxMax
		}

		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}

		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func MakePullUpdate() events.PullUpdate {
	return events.PullUpdate{
		Repo:      DefaultRepository,
		Action:    events.Opened,
		SHA:       RandSHA(),
		Number:    rand.Int(),
		Merged:    false,
		Timestamp: time.Now(),
	}
}

func MakeBranchUpdate() events.BranchUpdate {
	return events.BranchUpdate{
		Repo:   DefaultRepository,
		Action: events.Created,
		SHA:    RandSHA(),
		OldSHA: ZeroSHA,
	}
}
