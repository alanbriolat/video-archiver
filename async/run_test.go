package async

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	assert := assert.New(t)
	a := <-Run(func() int {
		return 123
	})
	assert.Equal(a, 123)
}

func TestRunResult(t *testing.T) {
	assert := assert.New(t)
	a := <-RunResult(func() (int, error) {
		return 123, nil
	})
	assert.Equal(a.Value, 123)
	assert.True(a.IsOk())
	b := <-RunResult(func() (int, error) {
		return 0, fmt.Errorf("error")
	})
	assert.True(b.IsErr())
}
