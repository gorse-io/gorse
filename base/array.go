package base

const batchSize = 1024 * 1024

type Integers struct {
	Data [][]int32
}

func (i *Integers) Len() int {
	if len(i.Data) == 0 {
		return 0
	}
	return len(i.Data)*batchSize - batchSize + len(i.Data[len(i.Data)-1])
}

func (i *Integers) Get(index int) int32 {
	return i.Data[index/batchSize][index%batchSize]
}

func (i *Integers) Append(val int32) {
	if len(i.Data) == 0 || len(i.Data[len(i.Data)-1]) == batchSize {
		i.Data = append(i.Data, make([]int32, 0, batchSize))
	}
	i.Data[len(i.Data)-1] = append(i.Data[len(i.Data)-1], val)
}
