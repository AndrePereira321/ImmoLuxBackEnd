package models

type RecordId int64

const InvalidRecordId = RecordId(-1)
const UnknownRecordId = RecordId(-2)

func (rid RecordId) IsValid() bool {
	return rid >= 0
}
