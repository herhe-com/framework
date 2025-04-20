package search

type Request struct {
	Offset    int
	Limit     int
	Condition string // 自定义条件
}
