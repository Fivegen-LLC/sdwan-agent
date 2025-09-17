package entities

type EmptyData struct{}

func NewEmptyData() EmptyData {
	return EmptyData{}
}

type InitFinishedData struct {
	err error
}

func NewInitFinishedData(err error) InitFinishedData {
	return InitFinishedData{
		err: err,
	}
}

func (d InitFinishedData) Err() error {
	return d.err
}
