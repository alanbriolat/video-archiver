package generic

type Set[T any] interface {
	Add(item T) bool
	Clear()
	Contains(items ...T) bool
	Clone() Set[T]
	Count() int
	//Difference(other Set[T]) Set[T]
	//Equal(other Set) bool
	//Intersection(other Set[T]) Set[T]
	//IsSubset(other Set) bool
	//IsSuperset(other Set) bool
	Remove(item T) bool
	//String() string
	ToSlice() []T
}

func NewSet[T comparable](items ...T) Set[T] {
	res := make(set[T])
	for _, item := range items {
		res.Add(item)
	}
	return &res
}

type set[T comparable] map[T]Void

func (s *set[T]) Add(item T) bool {
	_, found := (*s)[item]
	if found {
		return false
	}
	(*s)[item] = NewVoid()
	return true
}

func (s *set[T]) Clear() {
	*s = make(set[T])
}

func (s *set[T]) Clone() Set[T] {
	res := make(set[T])
	for item := range *s {
		res.Add(item)
	}
	return &res
}

func (s *set[T]) Contains(items ...T) bool {
	for _, item := range items {
		_, found := (*s)[item]
		if !found {
			return false
		}
	}
	return true
}

func (s *set[T]) Count() int {
	return len(*s)
}

func (s *set[T]) Remove(item T) bool {
	_, found := (*s)[item]
	if !found {
		return false
	}
	delete(*s, item)
	return true
}

func (s *set[T]) ToSlice() []T {
	slice := make([]T, 0, s.Count())
	for item := range *s {
		slice = append(slice, item)
	}
	return slice
}

type polymorphicSet[T any] map[interface{}]Void

func NewPolymorphicSet[T any](items ...T) Set[T] {
	res := make(polymorphicSet[T])
	for _, item := range items {
		res.Add(item)
	}
	return &res
}

func (s *polymorphicSet[T]) Add(item T) bool {
	_, found := (*s)[item]
	if found {
		return false
	}
	(*s)[item] = NewVoid()
	return true
}

func (s *polymorphicSet[T]) Clear() {
	*s = make(polymorphicSet[T])
}

func (s *polymorphicSet[T]) Contains(items ...T) bool {
	for _, item := range items {
		if _, found := (*s)[item]; !found {
			return false
		}
	}
	return true
}

func (s *polymorphicSet[T]) Clone() Set[T] {
	res := make(polymorphicSet[T])
	for item := range *s {
		res.Add(item.(T))
	}
	return &res
}

func (s *polymorphicSet[T]) Count() int {
	return len(*s)
}

func (s *polymorphicSet[T]) Remove(item T) bool {
	_, found := (*s)[item]
	if !found {
		return false
	}
	delete(*s, item)
	return true
}

func (s *polymorphicSet[T]) ToSlice() []T {
	slice := make([]T, 0, s.Count())
	for item := range *s {
		slice = append(slice, item.(T))
	}
	return slice
}
