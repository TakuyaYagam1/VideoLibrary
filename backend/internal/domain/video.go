package domain

type VideoID string

func (id VideoID) String() string {
	return string(id)
}

type Video struct {
	ID       VideoID
	Title    string
	FilePath string
	Views    int64
}
