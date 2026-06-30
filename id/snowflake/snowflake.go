package snowflake

import (
	"context"
	"fmt"

	bsnowflake "github.com/bwmarrin/snowflake"
)

const maxNodeID = int64(1023)

// Snowflake wraps a snowflake node as a Runa ID generator.
type Snowflake struct {
	node *bsnowflake.Node
}

// NewSnowflake creates a snowflake generator.
func NewSnowflake(nodeID int64) (*Snowflake, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("id node must be between 0 and %d", maxNodeID)
	}
	node, err := bsnowflake.NewNode(nodeID)
	if err != nil {
		return nil, err
	}
	return &Snowflake{node: node}, nil
}

// MustSnowflake creates a snowflake generator or panics.
func MustSnowflake(nodeID int64) *Snowflake {
	generator, err := NewSnowflake(nodeID)
	if err != nil {
		panic(err)
	}
	return generator
}

// New creates a new snowflake ID.
func (generator *Snowflake) New(ctx context.Context) (uint64, error) {
	if err := normalizeContext(ctx).Err(); err != nil {
		return 0, err
	}
	if generator == nil || generator.node == nil {
		return 0, fmt.Errorf("id generator is nil")
	}
	return uint64(generator.node.Generate().Int64()), nil
}

// String creates a new decimal string snowflake ID.
func (generator *Snowflake) String(ctx context.Context) (string, error) {
	if err := normalizeContext(ctx).Err(); err != nil {
		return "", err
	}
	if generator == nil || generator.node == nil {
		return "", fmt.Errorf("id generator is nil")
	}
	return generator.node.Generate().String(), nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
