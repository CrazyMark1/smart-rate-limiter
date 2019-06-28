package limit

type Queue []int64

// 向队列中添加一个元素
func (q *Queue) Push(v int64) {
	*q = append(*q, v)
}

// 从队列中删除第一个元素
func (q *Queue) Pop() int64 {
	head := (*q)[0]
	*q = (*q)[1:]
	return head
}
