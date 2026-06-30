package snowflake

import (
	"context"
	"sync"
	"testing"
)

func TestSnowflakeGeneratesUniqueIDs(t *testing.T) {
	generator := MustSnowflake(7)
	seen := sync.Map{}
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 200 {
				value, err := generator.New(context.Background())
				if err != nil {
					t.Errorf("new: %v", err)
					return
				}
				if _, loaded := seen.LoadOrStore(value, true); loaded {
					t.Errorf("duplicate id: %d", value)
					return
				}
			}
		}()
	}
	wait.Wait()
}
