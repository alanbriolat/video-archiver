package generic

type Void = struct{}

func NewVoid() Void {
	return Void{}
}
