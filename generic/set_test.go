package generic

import (
	"sort"
	"testing"

	assert_ "github.com/stretchr/testify/assert"
)

func TestSet(t *testing.T) {
	assert := assert_.New(t)

	s := NewSet[int]()
	assert.Equal(0, s.Count())
	assert.False(s.Contains(1))
	assert.True(s.Add(1))
	assert.Equal(1, s.Count())
	assert.True(s.Contains(1))
	assert.False(s.Add(1))
	assert.Equal(1, s.Count())
	assert.True(s.Contains(1))
	assert.True(s.Remove(1))
	assert.Equal(0, s.Count())
	assert.False(s.Contains(1))
	assert.False(s.Remove(1))
	assert.Equal(0, s.Count())
	assert.False(s.Contains(1))

	s2 := s.Clone()
	assert.True(s2.Add(1))
	assert.Equal(1, s2.Count())
	assert.True(s2.Contains(1))
	assert.False(s.Contains(1))

	s2.Clear()
	assert.False(s2.Contains(1))

	s3 := NewSet(1, 2, 3)
	assert.True(s3.Contains(3))
	items := s3.ToSlice()
	sort.Ints(items)
	assert.Equal([]int{1, 2, 3}, items)

	s4 := s3.Clone()
	items = s4.ToSlice()
	sort.Ints(items)
	assert.Equal([]int{1, 2, 3}, items)
}
