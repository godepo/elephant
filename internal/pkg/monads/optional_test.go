package monads

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmptyOf(t *testing.T) {
	empty := EmptyOf[string]()
	require.True(t, empty.IsEmpty())
	assert.Empty(t, empty.Value)
}

func TestOptionalOf(t *testing.T) {
	exp := uuid.NewString()
	empty := OptionalOf(exp)
	require.False(t, empty.IsEmpty())
	assert.Equal(t, exp, empty.Value)
}
